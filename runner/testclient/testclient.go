package testclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/pentops/flowtest/be"
)

type RequestLog struct {
	Method         string
	Path           string
	ResponseStatus int
	RequestBody    interface{}
	RequestHeaders http.Header
	ResponseBody   interface{}
	ResponseHeader http.Header
	Error          error
}

type API struct {
	BaseURL string

	Client *http.Client

	Logger func(*RequestLog)

	Auth AuthProvider
}

type AuthProvider interface {
	// Authenticate should set whatever headers etc are needed on the request.
	// It should cache results where a server call is involved, and it must not
	// attempt to authenticate where the request is already an auth request.
	Authenticate(req *http.Request) error
}

type BearerToken string

func (t BearerToken) Authenticate(req *http.Request) error {
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", t))
	return nil
}

func NewAPI(baseURL string) (*API, error) {
	_, err := url.Parse(baseURL)
	if err != nil {
		return nil, err
	}
	client := &http.Client{}

	api := &API{
		BaseURL: baseURL,
		Client:  client,
	}
	return api, nil
}

func (api *API) Request(ctx context.Context, method string, path string, body interface{}, response interface{}) error {

	logEntry := &RequestLog{
		Method:      method,
		Path:        path,
		RequestBody: body,
	}

	var bodyReader io.Reader
	if body != nil {
		switch method {
		case http.MethodPatch, http.MethodPost, http.MethodPut:

			bodyBytes, err := json.Marshal(body)
			if err != nil {
				return fmt.Errorf("marshalling request: %w", err)
			}
			bodyReader = bytes.NewReader(bodyBytes)
		default:
			return fmt.Errorf("unsupported method %s with body", method)
		}
	}

	fullURL := api.BaseURL + path
	fmt.Printf("Full URL %s\n", fullURL)
	req, err := http.NewRequest(method, fullURL, bodyReader)
	if err != nil {
		return err
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	if api.Auth != nil {
		err = api.Auth.Authenticate(req)
		if err != nil {
			return fmt.Errorf("authenticating: %w", err)
		}
	}

	logEntry.RequestHeaders = req.Header

	req = req.WithContext(ctx)

	resp, err := api.Client.Do(req)
	if err != nil {
		if api.Logger != nil {
			logEntry.Error = err
			api.Logger(logEntry)
		}
		return err
	}
	defer resp.Body.Close()

	logEntry.ResponseHeader = resp.Header

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		if api.Logger != nil {
			logEntry.Error = err
			api.Logger(logEntry)
		}
		return err
	}

	logEntry.ResponseStatus = resp.StatusCode
	logEntry.ResponseBody = bodyBytes

	if resp.StatusCode != http.StatusOK {
		if api.Logger != nil {
			api.Logger(logEntry)
		}

		return &APIError{
			StatusCode: resp.StatusCode,
		}
	}

	if response != nil {
		dd := json.NewDecoder(bytes.NewReader(bodyBytes))
		dd.DisallowUnknownFields()
		err = dd.Decode(response)
		if err != nil {
			if api.Logger != nil {
				logEntry.Error = err
				api.Logger(logEntry)
			}

			return fmt.Errorf("decoding API response: %w", err)
		}
		logEntry.ResponseBody = response
	}

	if api.Logger != nil {
		api.Logger(logEntry)
	}

	return nil
}

type APIError struct {
	StatusCode int
}

func (e *APIError) Error() string {
	return http.StatusText(e.StatusCode)
}

func failf(format string, args ...interface{}) *be.Outcome {
	fs := be.Outcome(fmt.Sprintf(format, args...))
	return &fs
}

func AssertHTTPError(err error, code int) *be.Outcome {
	if err == nil {
		return failf("expected HTTP error %d, got nil", code)
	}
	apiErr := &APIError{}
	if errors.As(err, &apiErr) {
		if apiErr.StatusCode != code {
			return failf("expected HTTP error %d, got %d", code, apiErr.StatusCode)
		}
	} else {
		return failf("expected HTTP error %d, got %s", code, err)
	}
	return nil
}

type PageRequest interface {
	SetPageToken(string)
}
type PageResponse[Item any] interface {
	GetPageToken() *string
	GetItems() []Item
}

func Paged[
	Req PageRequest,
	Res PageResponse[Item],
	Item any,
](ctx context.Context, baseReq Req, call func(context.Context, Req) (Res, error), callback func(Item) error) error {

	for {
		res, err := call(ctx, baseReq)
		if err != nil {
			return err
		}

		for _, item := range res.GetItems() {
			if err := callback(item); err != nil {
				if errors.Is(err, io.EOF) {
					return nil
				}
				return err
			}
		}

		resToken := res.GetPageToken()
		if resToken == nil {
			return nil
		}

		baseReq.SetPageToken(*resToken)
	}
}
