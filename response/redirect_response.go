package response

import "net/http"

type RedirectResponse struct {
	*BytesResponseWriter
}

//--------------------

func NewRedirectResponse(request *http.Request, url string, httpStatus int) *RedirectResponse {
	result := &RedirectResponse{
		BytesResponseWriter: NewBytesResponseWriter(),
	}

	http.Redirect(result, request, url, httpStatus)

	return result
}
