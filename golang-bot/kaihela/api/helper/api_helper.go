package helper

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"

	log "github.com/sirupsen/logrus"
)

type HttpMethod string
type ContentType string

const (
	MethodGet            HttpMethod  = "GET"
	MethodPost           HttpMethod  = "POST"
	ContentJSON          ContentType = "application/json"
	ContentFormUrlEncode ContentType = "application/x-www-form-urlencoded"
)

type ApiHelper struct {
	Token       string
	Type        string
	Language    string
	BaseUrl     string
	QueryParam  string
	Path        string
	Body        []byte
	ContentType ContentType
	Method      HttpMethod
}

func NewApiHelper(path, token, baseUrl, apiType, language string) *ApiHelper {
	apiHelper := &ApiHelper{Token: token, Type: "Bot", BaseUrl: "https://www.kaiheila.cn", Language: "zh-CN"}

	if baseUrl != "" {
		apiHelper.BaseUrl = baseUrl
	}
	if apiType != "" {
		apiHelper.Type = apiType
	}
	if language != "" {
		apiHelper.Language = language
	}
	apiHelper.Path = path
	apiHelper.ContentType = ContentJSON
	apiHelper.Method = MethodGet

	return apiHelper
}

func (h *ApiHelper) SetQuery(values map[string]string) {
	urlValues := url.Values{}
	for k, v := range values {
		urlValues.Add(k, v)
	}
	h.QueryParam = urlValues.Encode()
}

func (h *ApiHelper) SetBody(body []byte) *ApiHelper {
	h.Body = body
	return h
}

func (h *ApiHelper) SetContentType(contentType ContentType) {
	h.ContentType = contentType
}

func (h *ApiHelper) AddFile(fieldName, fileName string, fileData []byte) *ApiHelper {
	// 这里我们创建一个多部分表单的写入器
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// 添加文件部分
	part, err := writer.CreateFormFile(fieldName, fileName)
	if err != nil {
		log.Fatal(err)
	}
	part.Write(fileData)

	// 关闭写入器，这一步非常重要
	err = writer.Close()
	if err != nil {
		log.Fatal(err)
	}

	h.Body = body.Bytes()
	h.SetContentType(ContentType(writer.FormDataContentType())) // 设置正确的内容类型

	return h
}

func (h *ApiHelper) Get() ([]byte, error) {
	h.Method = MethodGet
	return h.Send()
}
func (h *ApiHelper) Post() ([]byte, error) {
	h.Method = MethodPost
	return h.Send()
}
func (h *ApiHelper) Send() ([]byte, error) {
	client := &http.Client{}
	reqPath := ""
	if strings.HasPrefix(h.Path, "/") || strings.HasSuffix(h.BaseUrl, "/") {
		reqPath = h.BaseUrl + h.Path
	} else {
		reqPath = h.BaseUrl + "/" + h.Path
	}
	if h.QueryParam != "" {
		reqPath += "?" + h.QueryParam
	}
	var req *http.Request
	var err error
	if h.Body != nil {
		req, err = http.NewRequest(string(h.Method), reqPath, bytes.NewBuffer(h.Body))
	} else {
		req, err = http.NewRequest(string(h.Method), reqPath, nil)
	}
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", string(h.ContentType))
	req.Header.Set("Authorization", fmt.Sprintf("%s %s", h.Type, h.Token))
	req.Header.Set("Accept-Language", h.Language)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		log.WithField("statusCode", resp.StatusCode).Error("http error", reqPath)
		return nil, errors.New("http error")
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func (h *ApiHelper) String() string {
	sb := strings.Builder{}
	sb.WriteString(fmt.Sprintf("Path:%s BaseUrl:%s Method:%s Query:%s ", h.Path, h.BaseUrl, h.Method, h.QueryParam))
	if len(h.Body) > 0 {
		sb.WriteString(fmt.Sprintf("Body:%s", string(h.Body)))
	}
	return sb.String()

}
