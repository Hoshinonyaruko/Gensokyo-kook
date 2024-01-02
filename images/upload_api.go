package images

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/hoshinonyaruko/gensokyo-kook/config"
	"github.com/hoshinonyaruko/gensokyo-kook/oss"
	"github.com/idodo/golang-bot/kaihela/api/helper"
)

// 将base64图片通过lotus转换成url
func UploadBase64ImageToServer(base64Image string) (string, error) {
	extraPicAuditingType := config.GetOssType()

	// 根据不同的extraPicAuditingType值来调整函数行为
	switch extraPicAuditingType {
	case 0:
		// 原有的函数行为
		return originalUploadBehavior(base64Image)
	case 1:
		return oss.UploadAndAuditImage(base64Image) //腾讯
	case 2:
		return oss.UploadAndAuditImageB(base64Image) //百度
	case 3:
		return oss.UploadAndAuditImageA(base64Image) //阿里
	default:
		return "", errors.New("invalid extraPicAuditingType")
	}
}

func originalUploadBehavior(base64Image string) (string, error) {
	// 原有的UploadBase64ImageToServer函数的实现
	protocol := "http"
	serverPort := config.GetPortValue()
	if serverPort == "443" {
		protocol = "https"
	}

	if config.GetLotusValue() {
		serverDir := config.GetServer_dir()
		url := fmt.Sprintf("%s://%s:%s/uploadpic", protocol, serverDir, serverPort)

		resp, err := postImageToServer(base64Image, url)
		if err != nil {
			return "", err
		}
		return resp, nil
	}

	serverDir := config.GetServer_dir()
	if serverPort == "443" {
		protocol = "http"
		serverPort = "444"
	}

	if isPublicAddress(serverDir) {
		url := fmt.Sprintf("%s://127.0.0.1:%s/uploadpic", protocol, serverPort)

		resp, err := postImageToServer(base64Image, url)
		if err != nil {
			return "", err
		}
		return resp, nil
	}
	return "", errors.New("local server uses a private address; image upload failed")
}

// 将base64语音通过lotus转换成url
func UploadBase64RecordToServer(base64Image string) (string, error) {
	// 根据serverPort确定协议
	protocol := "http"
	serverPort := config.GetPortValue()
	if serverPort == "443" {
		protocol = "https"
	}

	if config.GetLotusValue() {
		serverDir := config.GetServer_dir()
		url := fmt.Sprintf("%s://%s:%s/uploadrecord", protocol, serverDir, serverPort)

		resp, err := postRecordToServer(base64Image, url)
		if err != nil {
			return "", err
		}
		return resp, nil
	}

	serverDir := config.GetServer_dir()
	// 当端口是443时，使用HTTP和444端口
	if serverPort == "443" {
		protocol = "http"
		serverPort = "444"
	}

	if isPublicAddress(serverDir) {
		url := fmt.Sprintf("%s://127.0.0.1:%s/uploadrecord", protocol, serverPort)

		resp, err := postRecordToServer(base64Image, url)
		if err != nil {
			return "", err
		}
		return resp, nil
	}
	return "", errors.New("local server uses a private address; image record failed")
}

// 请求图床api(图床就是lolus为false的gensokyo-kook)
func postImageToServer(base64Image, targetURL string) (string, error) {
	data := url.Values{}
	data.Set("base64Image", base64Image) // 修改字段名以与服务器匹配

	resp, err := http.PostForm(targetURL, data)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("error response from server: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %v", err)
	}

	var responseMap map[string]interface{}
	if err := json.Unmarshal(body, &responseMap); err != nil {
		return "", fmt.Errorf("failed to unmarshal response: %v", err)
	}

	if value, ok := responseMap["url"]; ok {
		return fmt.Sprintf("%v", value), nil
	}

	return "", fmt.Errorf("URL not found in response")
}

// 请求语音床api(图床就是lolus为false的gensokyo-kook)
func postRecordToServer(base64Image, targetURL string) (string, error) {
	data := url.Values{}
	data.Set("base64Record", base64Image) // 修改字段名以与服务器匹配

	resp, err := http.PostForm(targetURL, data)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("error response from server: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %v", err)
	}

	var responseMap map[string]interface{}
	if err := json.Unmarshal(body, &responseMap); err != nil {
		return "", fmt.Errorf("failed to unmarshal response: %v", err)
	}

	if value, ok := responseMap["url"]; ok {
		return fmt.Sprintf("%v", value), nil
	}

	return "", fmt.Errorf("URL not found in response")
}

// 判断是否公网ip 填写域名也会被认为是公网,但需要用户自己确保域名正确解析到gensokyo-kook所在的ip地址
func isPublicAddress(addr string) bool {
	if strings.Contains(addr, "localhost") || strings.HasPrefix(addr, "127.") || strings.HasPrefix(addr, "192.168.") {
		return false
	}
	if net.ParseIP(addr) != nil {
		return true
	}
	// If it's not a recognized IP address format, consider it a domain name (public).
	return true
}

// ImageUploadResponse 用于解析上传后的响应
type ImageUploadResponse struct {
	Data struct {
		URL string `json:"url"`
	} `json:"data"`
}

// UploadImage 函数上传图片并返回图片URL
func UploadImage(filePath, token string, baseurl string) (string, error) {
	// 读取图片文件
	fileData, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}

	// 获取文件名（这里简单地使用完整路径的最后一部分）
	fileName := filepath.Base(filePath)

	// 创建ApiHelper实例
	apiHelper := helper.NewApiHelper("/api/v3/asset/create", token, baseurl, "", "")

	// 添加文件到ApiHelper
	apiHelper.AddFile("file", fileName, fileData)

	// 执行POST请求
	responseBody, err := apiHelper.Post()
	if err != nil {
		return "", err
	}

	// 解析响应JSON以获取图片URL
	var response ImageUploadResponse
	err = json.Unmarshal(responseBody, &response)
	if err != nil {
		return "", err
	}

	return response.Data.URL, nil
}

// UploadImageBase64 函数上传Base64编码的图片并返回图片URL
func UploadImageBase64(base64String, token string, baseurl string) (string, error) {
	// 替换所有空格为加号
	base64String = strings.ReplaceAll(base64String, " ", "+")
	// 解码Base64字符串
	decodedData, err := base64.StdEncoding.DecodeString(base64String)
	if err != nil {
		return "", err
	}

	// 生成随机文件名
	randomFileName := "temp_" + RandomString(10) + ".png"

	// 创建ApiHelper实例
	apiHelper := helper.NewApiHelper("/v3/asset/create", token, baseurl, "", "")

	// 添加文件到ApiHelper
	apiHelper.AddFile("file", randomFileName, decodedData)

	// 执行POST请求
	responseBody, err := apiHelper.Post()
	if err != nil {
		return "", err
	}

	// 解析响应JSON以获取图片URL
	var response ImageUploadResponse
	err = json.Unmarshal(responseBody, &response)
	if err != nil {
		return "", err
	}

	return response.Data.URL, nil
}

// RandomString 生成指定长度的随机字符串
func RandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}
