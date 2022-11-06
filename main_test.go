package main_test

import (
	"strings"
	"testing"

	fetch "github.com/eolme/go-fetch"
	promise "github.com/eolme/go-promise/promise"
)

const URL = "http://myhttpheader.com/"

func TestFetch(t *testing.T) {
	t.Run("single", func(t *testing.T) {
		_response, err := fetch.Fetch(URL, nil).Wait()
		if err != nil {
			t.Error(err.Error())
		}
		t.Log("response")
		response := _response.(*fetch.FetchResponse)
		t.Logf("ok `%t`", response.OK)
		t.Logf("status `%d`", response.Status)
		t.Logf("statusText `%s`", response.StatusText)
		t.Logf("url `%s`", response.URL)
		t.Logf("redirected `%t`", response.Redirected)
		for header, value := range *response.Headers {
			t.Logf("header `%s` `%s`", header, value)
		}
		_content, err := response.Text().Wait()
		if err != nil {
			t.Error(err.Error())
		}
		t.Log("content")
		content := _content.(string)
		t.Log(content)
	})

	t.Run("parallel", func(t *testing.T) {
		_responses, err := promise.All([]any{
			fetch.Fetch(URL, nil).Then(func(result any) (any, error) {
				t.Log(0, "done")
				return result, nil
			}),
			fetch.Fetch(URL, nil).Then(func(result any) (any, error) {
				t.Log(1, "done")
				return result, nil
			}),
			fetch.Fetch(URL, nil).Then(func(result any) (any, error) {
				t.Log(2, "done")
				return result, nil
			}),
			fetch.Fetch(URL, nil).Then(func(result any) (any, error) {
				t.Log(3, "done")
				return result, nil
			}),
		}).Wait()
		if err != nil {
			t.Error(err)
		}
		t.Log("responses")
		responses := _responses.([]any)
		for iter, _response := range responses {
			response := _response.(*fetch.FetchResponse)
			t.Log(iter, response.OK)
		}
	})

	t.Run("normalize", func(t *testing.T) {
		_response, _ := fetch.Fetch(URL, &fetch.FetchParams{
			Redirect: "error",
			Cache:    "no-store",
			Method:   "delete",
		}).Wait()
		response := _response.(*fetch.FetchResponse)
		_content, _ := response.Text().Wait()
		content := _content.(string)
		t.Log(content)
		if !strings.Contains(content, "DELETE / HTTP/1.1") ||
			!strings.Contains(content, "no-store") {
			t.Error("fail")
		}
	})

	t.Run("invalid", func(t *testing.T) {
		t.Parallel()

		t.Run("cache", func(t *testing.T) {
			_response, _ := fetch.Fetch(URL, &fetch.FetchParams{
				Cache: "invalid",
			}).Wait()
			response := _response.(*fetch.FetchResponse)
			_content, _ := response.Text().Wait()
			content := _content.(string)
			if !strings.Contains(content, "GET / HTTP/1.1") {
				t.Error("fail")
			}
		})

		t.Run("redirect", func(t *testing.T) {
			_response, _ := fetch.Fetch(URL, &fetch.FetchParams{
				Redirect: "invalid",
			}).Wait()
			response := _response.(*fetch.FetchResponse)
			_content, _ := response.Text().Wait()
			content := _content.(string)
			if !strings.Contains(content, "GET / HTTP/1.1") {
				t.Error("fail")
			}
		})

		t.Run("method", func(t *testing.T) {
			_response, _ := fetch.Fetch(URL, &fetch.FetchParams{
				Method: "invalid",
			}).Wait()
			response := _response.(*fetch.FetchResponse)
			_content, _ := response.Text().Wait()
			content := _content.(string)
			if !strings.Contains(content, "GET / HTTP/1.1") {
				t.Error("fail")
			}
		})
	})
}
