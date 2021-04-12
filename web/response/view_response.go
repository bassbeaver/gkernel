package response

import (
	"bytes"
	"html/template"
	"net/http"
)

type ViewResponse struct {
	basicResponse
	templateName string
	*template.Template
	data interface{}
}

func (r *ViewResponse) GetBodyBytes() *bytes.Buffer {
	if nil == r.Template {
		panic("Failed to render view, no template object set for template " + r.templateName)
	}

	result := bytes.NewBuffer(make([]byte, 0))

	err := r.Template.ExecuteTemplate(result, r.templateName, r.data)
	if err != nil {
		panic(err)
	}

	return result
}

func (r *ViewResponse) SetTemplate(tpl *template.Template) {
	r.Template = tpl
}

func (r *ViewResponse) SetData(data interface{}) {
	r.data = data
}

//--------------------

func NewViewResponse(templateName string) *ViewResponse {
	r := &ViewResponse{
		basicResponse: basicResponse{
			httpStatus: http.StatusOK,
			headers:    make(http.Header),
		},
		templateName: templateName,
		data:         nil,
	}
	r.HeaderSet("Content-Type", "text/html")

	return r
}
