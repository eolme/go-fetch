package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	panics "github.com/eolme/go-promise/panics"
	promise "github.com/eolme/go-promise/promise"
	fasthttp "github.com/valyala/fasthttp"
)

type internalBodyType = int

const (
	internalBodyUnsupported = 0
	internalBodyString      = 1
	internalBodyRaw         = 2
	internalBodyStream      = 3
	internalBodyWriter      = 4
)

type FetchCache = string

const (
	FetchCacheDefault = "default"
	FetchCacheNoCache = "no-cache"
	FetchCacheNoStore = "no-store"
)

type FetchRedirect = string

const (
	FetchRedirectFollow = "follow"
	FetchRedirectError  = "error"
)

type FetchBody struct {
	valueType internalBodyType
	value     any
	length    int
}

type FetchParams struct {
	Method   string
	Redirect FetchRedirect
	Cache    FetchCache
	Body     *FetchBody
	Headers  *map[string]string
}

func normalizeParams(params *FetchParams) *FetchParams {
	if params == nil {
		params = &FetchParams{}
	}

	switch params.Cache {
	case FetchCacheDefault:
	case FetchCacheNoCache:
	case FetchCacheNoStore:
	default:
		params.Cache = FetchCacheDefault
	}

	switch params.Redirect {
	case FetchRedirectError:
	case FetchRedirectFollow:
	default:
		params.Redirect = FetchRedirectFollow
	}

	params.Method = strings.ToUpper(params.Method)

	switch params.Method {
	case fasthttp.MethodGet:
	case fasthttp.MethodHead:
	case fasthttp.MethodPost:
	case fasthttp.MethodPut:
	case fasthttp.MethodPatch:
	case fasthttp.MethodDelete:
	case fasthttp.MethodConnect:
	case fasthttp.MethodOptions:
	case fasthttp.MethodTrace:
	default:
		params.Method = fasthttp.MethodGet
	}

	return params
}

type FetchResponse struct {
	OK         bool
	Redirected bool
	Status     int
	StatusText string
	URL        string
	BodyUsed   bool
	body       *[]byte
	Headers    *map[string]string
}

// Promise with *[]byte
func (self *FetchResponse) Raw() *promise.Promise {
	pointer := self.body
	self.body = nil
	self.BodyUsed = true

	return promise.Resolve(pointer)
}

// Promise with string
func (self *FetchResponse) Text() *promise.Promise {
	pointer := self.body
	self.body = nil
	self.BodyUsed = true

	return promise.Resolve(string(*pointer))
}

// Promise with v as-is
func (self *FetchResponse) Json(v any) *promise.Promise {
	pointer := self.body
	self.body = nil
	self.BodyUsed = true

	return panics.PromisifyPanic(func() any {
		json.Unmarshal(*pointer, v)
		return v
	})
}

// Promise with *bytes.Reader
func (self *FetchResponse) Body() *promise.Promise {
	pointer := self.body
	self.body = nil
	self.BodyUsed = true

	reader := bytes.NewReader(*pointer)
	return promise.Resolve(&reader)
}

func NewBody(value any, length int) (body *FetchBody) {
	body = &FetchBody{
		valueType: internalBodyUnsupported,
		value:     nil,
		length:    length,
	}

	if valueString, ok := value.(string); ok {
		body.valueType = internalBodyString
		body.value = valueString
		return body
	}

	if valueRaw, ok := value.([]byte); ok {
		body.valueType = internalBodyRaw
		body.value = valueRaw
		return body
	}

	if valueStream, ok := value.(io.Reader); ok {
		if length == 0 {
			return body
		}

		body.valueType = internalBodyStream
		body.value = valueStream
		return body
	}

	if valueWriter, ok := value.(fasthttp.StreamWriter); ok {
		body.valueType = internalBodyWriter
		body.value = valueWriter
		return body
	}

	return body
}

func Fetch(url string, params *FetchParams) *promise.Promise {
	return promise.New(func(resolve promise.PromiseResolve, reject promise.PromiseReject) {
		go func() {
			request := fasthttp.AcquireRequest()
			defer fasthttp.ReleaseRequest(request)

			response := fasthttp.AcquireResponse()
			defer fasthttp.ReleaseResponse(response)

			params = normalizeParams(params)

			request.SetRequestURI(url)
			request.Header.SetMethod(params.Method)

			if params.Headers != nil {
				for header, value := range *params.Headers {
					request.Header.Set(header, value)
				}
			}

			if params.Cache != FetchCacheDefault {
				request.Header.Set("Cache-Control", params.Cache)
			}

			if params.Body != nil {
				switch params.Body.valueType {
				case internalBodyUnsupported:
					reject(errors.New(fmt.Sprintf("Invalid body: %#v", params.Body.value)))
					return
				case internalBodyString:
					request.SetBodyString(params.Body.value.(string))
				case internalBodyRaw:
					request.SetBodyRaw(params.Body.value.([]byte))
				case internalBodyStream:
					request.SetBodyStream(params.Body.value.(io.Reader), params.Body.length)
				case internalBodyWriter:
					request.SetBodyStreamWriter(params.Body.value.(fasthttp.StreamWriter))
				}
			}

			if params.Redirect == FetchRedirectError {
				err := fasthttp.Do(request, response)
				if err != nil {
					reject(err)
					return
				}
			} else {
				err := fasthttp.DoRedirects(request, response, 10)
				if err != nil {
					reject(err)
					return
				}
			}

			status := response.StatusCode()
			redirected := false

			if params.Redirect == FetchRedirectError {
				redirected = status >= 300 && status < 400
			}

			headers := make(map[string]string, response.Header.Len())

			response.Header.VisitAll(func(key, value []byte) {
				headers[string(key)] = string(value)
			})

			result := &FetchResponse{
				OK:         status >= 200 && status < 300,
				Status:     status,
				StatusText: fasthttp.StatusMessage(status),
				Redirected: redirected,
				URL:        request.URI().String(),
				Headers:    &headers,
			}

			body := response.Body()
			safe := make([]byte, len(body))
			copy(safe, body)

			result.body = &safe

			resolve(result)
		}()
	})
}
