package gemini

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gabriel-vasile/mimetype"
	gemini "google.golang.org/genai"

	"github.com/bububa/instructor-go"
)

func ConvertMessageFrom(src *instructor.Message, dist *gemini.Content) error {
	if len(src.ToolUses) > 0 {
		list := make([]*gemini.Part, 0, len(src.ToolUses))
		for _, v := range src.ToolUses {
			args := make(map[string]any)
			if err := json.Unmarshal([]byte(v.Arguments), &args); err != nil {
				continue
			}
			part := gemini.NewPartFromFunctionCall(v.Name, args)
			list = append(list, part)
		}
		dist.Role = gemini.RoleModel
		dist.Parts = list
		return nil
	}
	if len(src.ToolResults) > 0 {
		list := make([]*gemini.Part, 0, len(src.ToolResults))
		for _, v := range src.ToolResults {
			args := make(map[string]any)
			if err := json.Unmarshal([]byte(v.Content), &args); err != nil {
				continue
			}
			part := gemini.NewPartFromFunctionResponse(v.Name, args)
			list = append(list, part)
		}
		dist.Role = gemini.RoleModel
		dist.Parts = list
		return nil
	}
	if src.Role != instructor.UserRole && src.Role != instructor.AssistantRole {
		return errors.New("do not support role")
	}
	list := make([]*gemini.Part, 0, len(src.Files)+len(src.Audios)+len(src.Images)+1)
	var buf bytes.Buffer
	for _, v := range src.Files {
		mediaTypeStr := "text/plain"
		data, err := base64.StdEncoding.DecodeString(v.Data)
		if err != nil {
			continue
		}
		buf.Reset()
		buf.Write(data)
		if mediaType, err := mimetype.DetectReader(&buf); err == nil {
			mediaTypeStr = mediaType.String()
		}
		part := gemini.NewPartFromBytes(data, mediaTypeStr)
		list = append(list, part)
	}
	for _, v := range src.Audios {
		data, err := base64.StdEncoding.DecodeString(v.Data)
		if err != nil {
			continue
		}
		mediaTypeStr := fmt.Sprintf("audio/%s", v.Format)
		if v.Format == "" {
			buf.Reset()
			buf.Write(data)
			if mediaType, err := mimetype.DetectReader(&buf); err == nil {
				mediaTypeStr = mediaType.String()
			}
		}
		part := gemini.NewPartFromBytes(data, mediaTypeStr)
		list = append(list, part)
	}
	for _, v := range src.Videos {
		buf.Reset()
		if err := DataFromURL(v.URL, &buf); err != nil {
			continue
		}
		bs := buf.Bytes()
		mediaTypeStr := "video/mp4"
		mediaType, err := mimetype.DetectReader(&buf)
		if err == nil {
			mediaTypeStr = mediaType.String()
		}
		part := gemini.NewPartFromBytes(bs, mediaTypeStr)
		list = append(list, part)
	}
	for _, v := range src.Images {
		buf.Reset()
		if err := DataFromURL(v.URL, &buf); err != nil {
			continue
		}
		bs := buf.Bytes()
		mediaTypeStr := "image/jpeg"
		mediaType, err := mimetype.DetectReader(&buf)
		if err == nil {
			mediaTypeStr = mediaType.String()
		}
		part := gemini.NewPartFromBytes(bs, mediaTypeStr)
		list = append(list, part)
	}
	if src.Text != "" {
		list = append(list, gemini.NewPartFromText(src.Text))
	}
	switch src.Role {
	case instructor.UserRole:
		dist.Role = gemini.RoleUser
	case instructor.AssistantRole:
		dist.Role = gemini.RoleModel
	}
	dist.Parts = list
	return nil
}

func ConvertMessageTo(src *gemini.Content, dist *instructor.Message) {
	switch src.Role {
	case gemini.RoleModel:
		dist.Role = instructor.AssistantRole
	case gemini.RoleUser:
		dist.Role = instructor.UserRole
	}
	var buf bytes.Buffer
	for _, part := range src.Parts {
		if call := part.FunctionCall; call != nil {
			bs, _ := json.Marshal(call.Args)
			dist.ToolUses = append(dist.ToolUses, instructor.ToolUse{
				ID:        call.ID,
				Name:      call.Name,
				Arguments: string(bs),
			})
		} else if tool := part.FunctionResponse; tool != nil {
			bs, _ := json.Marshal(tool.Response)
			dist.ToolResults = append(dist.ToolResults, instructor.ToolResult{
				ID:      tool.ID,
				Name:    tool.Name,
				Content: string(bs),
			})
		} else {
			if source := part.FileData; source != nil {
				if strings.HasPrefix(source.MIMEType, "image") {
					dist.Images = append(dist.Images, instructor.Image{
						URL: source.FileURI,
					})
				} else if strings.HasPrefix(source.MIMEType, "video") {
					dist.Videos = append(dist.Videos, instructor.Video{
						URL: source.FileURI,
					})
				} else if strings.HasPrefix(source.MIMEType, "audio") {
					buf.Reset()
					if err := DataFromURL(source.FileURI, &buf); err == nil {
						data := base64.StdEncoding.EncodeToString(buf.Bytes())
						dist.Audios = append(dist.Audios, instructor.Audio{
							Data:   data,
							Format: strings.TrimPrefix(source.MIMEType, "audio/"),
						})
					}
				} else {
					buf.Reset()
					if err := DataFromURL(source.FileURI, &buf); err == nil {
						data := base64.StdEncoding.EncodeToString(buf.Bytes())
						dist.Files = append(dist.Files, instructor.File{
							Data: data,
							Name: source.DisplayName,
						})
					}
				}
			}
			if source := part.InlineData; source != nil {
				data := base64.StdEncoding.EncodeToString(source.Data)
				uri := fmt.Sprintf("data:%s;base64,%s", source.MIMEType, data)
				if strings.HasPrefix(source.MIMEType, "image") {
					dist.Images = append(dist.Images, instructor.Image{
						URL: uri,
					})
				} else if strings.HasPrefix(source.MIMEType, "video") {
					dist.Videos = append(dist.Videos, instructor.Video{
						URL: uri,
					})
				} else if strings.HasPrefix(source.MIMEType, "audio") {
					data := base64.StdEncoding.EncodeToString(buf.Bytes())
					dist.Audios = append(dist.Audios, instructor.Audio{
						Data:   data,
						Format: uri,
					})
				} else {
					dist.Files = append(dist.Files, instructor.File{
						Data: data,
						Name: source.DisplayName,
					})
				}
			}
			if part.Text != "" {
				dist.Text = part.Text
			}
		}
	}
}

func DataFromURL(link string, w io.Writer) error {
	if strings.HasPrefix(link, "data:") && strings.Contains(link, ";base64,") {
		b64 := link
		parts := strings.Split(b64, ",")
		if len(parts) == 2 {
			b64 = strings.TrimSpace(parts[1])
		}
		if bs, err := base64.StdEncoding.DecodeString(b64); err != nil {
			return err
		} else {
			w.Write(bs)
		}
		return nil
	}
	if !strings.HasPrefix(link, "http://") && !strings.HasPrefix(link, "https://") {
		return errors.New("invalid link")
	}
	resp, err := http.Get(link)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	io.Copy(w, resp.Body)
	return nil
}
