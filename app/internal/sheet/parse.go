package sheet

import (
	"context"
	"encoding/base64"
	"fmt"
	"mime"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/schema"
)

// Config points at any OpenAI-compatible chat endpoint.
type Config struct {
	BaseURL string
	APIKey  string
	Model   string
}

// Ready reports whether the endpoint has enough information to call.
func (c Config) Ready() bool {
	key := strings.TrimSpace(c.APIKey)
	base := strings.TrimSpace(c.BaseURL)
	return strings.TrimSpace(c.Model) != "" && (key != "" || base != "")
}

// Client parses chart images through a vision-capable chat model.
type Client struct {
	model *openai.ChatModel
}

func New(ctx context.Context, cfg Config) (*Client, error) {
	if !cfg.Ready() {
		return nil, fmt.Errorf("请先在设置里完成图片识别配置")
	}
	model, err := openai.NewChatModel(ctx, &openai.ChatModelConfig{
		BaseURL: strings.TrimSpace(cfg.BaseURL),
		APIKey:  strings.TrimSpace(cfg.APIKey),
		Model:   strings.TrimSpace(cfg.Model),
		Timeout: 2 * time.Minute,
	})
	if err != nil {
		return nil, err
	}
	return &Client{model: model}, nil
}

const prompt = `你是尤克里里曲谱 OCR。把图片里的和弦谱转成严格 JSON，不要任何解释：{"key":"C","capo":0,"bpm":0,"timeSignature":[4,4],"lines":[{"chords":"F      G   Em7   Am","lyrics":"好想能看到 你嘴角微笑"}]}。要求：按谱面从上到下逐行输出；和弦行原样放进 chords 字符串，尽量保留和弦之间的空格和列位置；歌词行放在 lyrics；不要计算字符下标，不要输出 at；纯和弦行也要输出，lyrics 用空字符串；chords 保留原始写法（如 Em7、A7、Bb），不要移调；忽略二维码、水印、和弦图、页脚、标题；重复段落要逐行输出，不要省略；如果谱面写了 BPM/速度，填到 bpm，否则 bpm 用 0；如果谱面写了拍号，填到 timeSignature，否则用 [4,4]；如果看不到调性或 capo，key 用空字符串，capo 用 0；只输出 JSON。`

// ParseImage reads a chart image and returns its structured lines.
func (c *Client) ParseImage(ctx context.Context, path string) (*Sheet, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return c.ParseImageData(ctx, data, imageMIME(path))
}

// ParseImageData parses image bytes, as read from a file or the clipboard.
func (c *Client) ParseImageData(ctx context.Context, data []byte, mimeType string) (*Sheet, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("图片为空")
	}
	if strings.TrimSpace(mimeType) == "" {
		mimeType = "image/png"
	}
	encoded := base64.StdEncoding.EncodeToString(data)
	message := &schema.Message{
		Role: schema.User,
		UserInputMultiContent: []schema.MessageInputPart{
			{Type: schema.ChatMessagePartTypeText, Text: prompt},
			{
				Type: schema.ChatMessagePartTypeImageURL,
				Image: &schema.MessageInputImage{
					MessagePartCommon: schema.MessagePartCommon{
						Base64Data: &encoded,
						MIMEType:   mimeType,
					},
					Detail: schema.ImageURLDetailHigh,
				},
			},
		},
	}
	out, err := c.model.Generate(ctx, []*schema.Message{message})
	if err != nil {
		return nil, fmt.Errorf("图片识别失败，请确认设置里的模型支持图片输入：%w", err)
	}
	if out == nil || strings.TrimSpace(out.Content) == "" {
		return nil, fmt.Errorf("图片识别没有返回内容")
	}
	return Decode(out.Content)
}

func imageMIME(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	if value := mime.TypeByExtension(ext); value != "" {
		return value
	}
	switch ext {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".webp":
		return "image/webp"
	default:
		return "image/png"
	}
}
