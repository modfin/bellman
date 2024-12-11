package prompt

import "encoding/base64"

type Role string

const User = Role("user")
const Assistant = Role("assistant")

type Prompt struct {
	Role    Role     `json:"role"`
	Text    string   `json:"text"`
	Payload *Payload `json:"payload,omitempty"`
}

type Payload struct {
	Mime string `json:"mime_type"`
	Data string `json:"data"`
}

func AsAssistant(text string) Prompt {
	return Prompt{Role: Assistant, Text: text}
}
func AsUser(text string) Prompt {
	return Prompt{Role: User, Text: text}
}
func AsUserWithData(mime string, data []byte) Prompt {
	return Prompt{Role: User, Payload: &Payload{Mime: mime, Data: base64.StdEncoding.EncodeToString(data)}}
}

const MimeApplicationPDF = "application/pdf"
const MimeAudioMPEG = "audio/mpeg"
const MimeAudioMP3 = "audio/mp3"
const MimeAudioWAV = "audio/wav"
const MimeImagePNG = "image/png"
const MimeImageJPEG = "image/jpeg"
const MimeImageWebp = "image/webp"
const MimeTextPlain = "text/plain"
const MimeVideoMOV = "video/mov"
const MimeVideoMPEG = "video/mpeg"
const MimeVideoMP4 = "video/mp4"
const MimeVideoMPG = "video/mpg"
const MimeVideoAVI = "video/avi"
const MimeVideoWMV = "video/wmv"
const MimeVideoMPEGS = "video/mpegps"
const MimeVideoFLV = "video/flv"
