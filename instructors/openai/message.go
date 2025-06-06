package openai

import (
	"errors"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/shared/constant"

	"github.com/bububa/instructor-go"
)

func ConvertMessageFrom(src *instructor.Message) []openai.ChatCompletionMessageParamUnion {
	if src.Role == instructor.SystemRole {
		if src.Text != "" {
			return []openai.ChatCompletionMessageParamUnion{openai.SystemMessage(src.Text)}
		}
		return nil
	}
	if len(src.ToolResults) > 0 {
		list := make([]openai.ChatCompletionMessageParamUnion, 0, len(src.ToolResults))
		for _, v := range src.ToolResults {
			msg := openai.ToolMessage(v.Content, v.ID)
			list = append(list, msg)
		}
		return list
	}
	if len(src.ToolUses) > 0 {
		list := make([]openai.ChatCompletionMessageToolCallParam, 0, len(src.ToolUses))
		for _, v := range src.ToolUses {
			call := openai.ChatCompletionMessageToolCallParam{
				ID: v.ID,
				Function: openai.ChatCompletionMessageToolCallFunctionParam{
					Name:      v.Name,
					Arguments: v.Arguments,
				},
			}
			list = append(list, call)
		}
		msg := openai.ChatCompletionMessageParamUnion{
			OfAssistant: &openai.ChatCompletionAssistantMessageParam{
				ToolCalls: list,
			},
		}
		return []openai.ChatCompletionMessageParamUnion{msg}
	}
	list := make([]openai.ChatCompletionContentPartUnionParam, 0, len(src.Files)+len(src.Audios)+len(src.Images)+1)
	if len(src.Files) > 0 {
		for _, v := range src.Files {
			fp := openai.FileContentPart(openai.ChatCompletionContentPartFileFileParam{
				FileData: openai.String(v.Data),
				FileID:   openai.String(v.ID),
				Filename: openai.String(v.Name),
			})
			list = append(list, fp)
		}
	}
	if len(src.Audios) > 0 {
		msgs := make([]openai.ChatCompletionMessageParamUnion, 0, len(src.Audios))
		for _, v := range src.Audios {
			if src.Role == instructor.AssistantRole {
				msg := openai.ChatCompletionMessageParamUnion{
					OfAssistant: &openai.ChatCompletionAssistantMessageParam{
						Audio: openai.ChatCompletionAssistantMessageParamAudio{
							ID: v.ID,
						},
					},
				}
				msgs = append(msgs, msg)
				continue
			}
			fp := openai.InputAudioContentPart(openai.ChatCompletionContentPartInputAudioInputAudioParam{
				Data:   v.Data,
				Format: v.Format,
			})
			list = append(list, fp)
		}
		if len(msgs) > 0 {
			return msgs
		}
	}
	if len(src.Videos) > 0 {
		for _, v := range src.Videos {
			videoParam := &openai.ChatCompletionContentPartImageParam{
				Type: constant.ImageURL("video_url"),
			}
			videoParam.SetExtraFields(map[string]any{
				"video_url": openai.ImageURL{
					URL:    v.URL,
					Detail: openai.ImageURLDetail(v.Detail),
				},
			})

			part := openai.ChatCompletionContentPartUnionParam{
				OfImageURL: videoParam,
			}
			list = append(list, part)
		}
	}
	if len(src.Images) > 0 {
		for _, v := range src.Images {
			fp := openai.ImageContentPart(openai.ChatCompletionContentPartImageImageURLParam{
				URL:    v.URL,
				Detail: v.Detail,
			})
			list = append(list, fp)
		}
	}
	if src.Text != "" {
		if src.Role == instructor.AssistantRole {
			return []openai.ChatCompletionMessageParamUnion{openai.AssistantMessage(src.Text)}
		}
		list = append(list, openai.TextContentPart(src.Text))
	}
	if len(list) == 0 {
		return nil
	}
	return []openai.ChatCompletionMessageParamUnion{openai.UserMessage(list)}
}

func ConvertMessageTo(src *openai.ChatCompletionMessageParamUnion, dist *instructor.Message) error {
	if msg := src.OfAssistant; msg != nil {
		dist.Role = instructor.AssistantRole
		if len(msg.ToolCalls) > 0 {
			for _, call := range msg.ToolCalls {
				dist.ToolUses = append(dist.ToolUses, instructor.ToolUse{
					ID:        call.ID,
					Name:      call.Function.Name,
					Arguments: call.Function.Arguments,
				})
			}
		} else if msg.Audio.ID != "" {
			dist.Audios = append(dist.Audios, instructor.Audio{
				ID: msg.Audio.ID,
			})
		} else if txt := msg.Content.OfString.Value; txt != "" {
			dist.Text = txt
		} else {
			return errors.New("invalid assistant message")
		}
	} else if msg := src.OfUser; msg != nil {
		dist.Role = instructor.UserRole
		if text := msg.Content.OfString.Value; text != "" {
			dist.Text = text
		}
		if len(msg.Content.OfArrayOfContentParts) > 0 {
			for _, part := range msg.Content.OfArrayOfContentParts {
				if v := part.OfFile; v != nil {
					dist.Files = append(dist.Files, instructor.File{
						ID:   v.File.FileID.Value,
						Name: v.File.Filename.Value,
						Data: v.File.FileData.Value,
					})
				} else if v := part.OfInputAudio; v != nil {
					dist.Audios = append(dist.Audios, instructor.Audio{
						Data:   v.InputAudio.Data,
						Format: v.InputAudio.Format,
					})
				} else if v := part.OfImageURL; v != nil {
					if v.Type == constant.ImageURL("video_url") {
						dist.Videos = append(dist.Videos, instructor.Video{
							URL:    v.ImageURL.URL,
							Detail: v.ImageURL.Detail,
						})
					}
					dist.Images = append(dist.Images, instructor.Image{
						URL:    v.ImageURL.URL,
						Detail: v.ImageURL.Detail,
					})
				} else if v := part.OfText; v != nil {
					dist.Text = v.Text
				}
			}
		}
	} else if msg := src.OfTool; msg != nil {
		dist.Role = instructor.ToolRole
		dist.ToolResults = append(dist.ToolResults, instructor.ToolResult{
			ID:      msg.ToolCallID,
			Content: msg.Content.OfString.Value,
		})
	} else {
		return errors.New("role not support")
	}
	return nil
}
