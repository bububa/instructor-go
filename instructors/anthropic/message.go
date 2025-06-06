package anthropic

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
	anthropic "github.com/liushuangls/go-anthropic/v2"

	"github.com/bububa/instructor-go"
)

func ConvertMessageFrom(src *instructor.Message, dist *anthropic.Message) error {
	if len(src.ToolUses) > 0 {
		list := make([]anthropic.MessageContent, 0, len(src.ToolUses))
		for _, v := range src.ToolUses {
			part := anthropic.NewToolUseMessageContent(v.ID, v.Name, []byte(v.Arguments))
			list = append(list, part)
		}
		dist.Role = anthropic.RoleAssistant
		dist.Content = list
		return nil
	}
	if len(src.ToolResults) > 0 {
		list := make([]anthropic.MessageContent, 0, len(src.ToolResults))
		for _, v := range src.ToolResults {
			part := anthropic.NewToolResultMessageContent(v.ID, v.Content, v.IsError)
			list = append(list, part)
		}
		dist.Role = anthropic.RoleUser
		dist.Content = list
		return nil
	}
	if src.Role != instructor.UserRole && src.Role != instructor.AssistantRole {
		return errors.New("do not support role")
	}
	list := make([]anthropic.MessageContent, 0, len(src.Files)+len(src.Audios)+len(src.Images)+1)
	var buf bytes.Buffer
	for _, v := range src.Files {
		mediaTypeStr := "text/plain"
		if data, err := base64.StdEncoding.DecodeString(v.Data); err == nil {
			buf.Reset()
			buf.Write(data)
			if mediaType, err := mimetype.DetectReader(&buf); err == nil {
				mediaTypeStr = mediaType.String()
			}
		}
		msg := anthropic.NewDocumentMessageContent(anthropic.NewMessageContentSource(anthropic.MessagesContentSourceTypeBase64, mediaTypeStr, v.Data), v.Name, "", false)
		list = append(list, msg)
	}
	for _, v := range src.Audios {
		mediaTypeStr := fmt.Sprintf("audio/%s", v.Format)
		if v.Format == "" {
			if data, err := base64.StdEncoding.DecodeString(v.Data); err == nil {
				buf.Reset()
				buf.Write(data)
				if mediaType, err := mimetype.DetectReader(&buf); err == nil {
					mediaTypeStr = mediaType.String()
				}
			}
		}
		msg := anthropic.NewDocumentMessageContent(anthropic.NewMessageContentSource(anthropic.MessagesContentSourceTypeBase64, mediaTypeStr, v.Data), "", "", false)
		list = append(list, msg)
	}
	for _, v := range src.Videos {
		buf.Reset()
		if err := DataFromURL(v.URL, &buf); err != nil {
			continue
		}
		data := base64.StdEncoding.EncodeToString(buf.Bytes())
		mediaTypeStr := "video/mp4"
		mediaType, err := mimetype.DetectReader(&buf)
		if err == nil {
			mediaTypeStr = mediaType.String()
		}
		msg := anthropic.NewDocumentMessageContent(anthropic.NewMessageContentSource(anthropic.MessagesContentSourceTypeBase64, mediaTypeStr, data), "", "", false)
		list = append(list, msg)
	}
	for _, v := range src.Images {
		buf.Reset()
		if err := DataFromURL(v.URL, &buf); err != nil {
			continue
		}
		data := base64.StdEncoding.EncodeToString(buf.Bytes())
		mediaTypeStr := "image/jpeg"
		mediaType, err := mimetype.DetectReader(&buf)
		if err == nil {
			mediaTypeStr = mediaType.String()
		}
		msg := anthropic.NewImageMessageContent(anthropic.NewMessageContentSource(anthropic.MessagesContentSourceTypeBase64, mediaTypeStr, data))
		list = append(list, msg)
	}
	if src.Text != "" {
		list = append(list, anthropic.NewTextMessageContent(src.Text))
	}
	switch src.Role {
	case instructor.UserRole:
		dist.Role = anthropic.RoleUser
	case instructor.AssistantRole:
		dist.Role = anthropic.RoleAssistant
	}
	dist.Content = list
	return nil
}

func ConvertMessageTo(src *anthropic.Message, dist *instructor.Message) error {
	switch src.Role {
	case anthropic.RoleAssistant:
		dist.Role = instructor.AssistantRole
	case anthropic.RoleUser:
		dist.Role = instructor.UserRole
	}
	for _, content := range src.Content {
		if call := content.MessageContentToolUse; call != nil {
			bs, _ := json.Marshal(call.Input)
			dist.ToolUses = append(dist.ToolUses, instructor.ToolUse{
				ID:        call.ID,
				Name:      call.Name,
				Arguments: string(bs),
			})
		} else if tool := content.MessageContentToolResult; tool != nil {
			toolRet := instructor.ToolResult{
				ID:      *tool.ToolUseID,
				Content: *tool.Content[0].Text,
			}
			if tool.IsError != nil {
				toolRet.IsError = *tool.IsError
			}
			dist.ToolResults = append(dist.ToolResults, toolRet)
		} else {
			if source := content.Source; source != nil {
				if strings.HasPrefix(source.MediaType, "image") {
					dist.Images = append(dist.Images, instructor.Image{
						URL: fmt.Sprintf("data:%s;base64,%s", source.MediaType, source.Data),
					})
				} else if strings.HasPrefix(source.MediaType, "video") {
					dist.Videos = append(dist.Videos, instructor.Video{
						URL: fmt.Sprintf("data:%s;base64,%s", source.MediaType, source.Data),
					})
				} else if strings.HasPrefix(source.MediaType, "audio") {
					if data, ok := source.Data.(string); ok {
						dist.Audios = append(dist.Audios, instructor.Audio{
							Data:   data,
							Format: strings.TrimPrefix(source.MediaType, "audio/"),
						})
					}
				} else if data, ok := source.Data.(string); ok {
					dist.Files = append(dist.Files, instructor.File{
						Data: data,
					})
				}
			}
			if text := content.Text; text != nil {
				dist.Text = *text
			}
		}
	}
	return nil
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
