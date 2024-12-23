package prompt

import "encoding/base64"

type Role string

const System = Role("system")
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
	Uri  string `json:"uri"`
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
func AsUserWithURI(mime string, uri string) Prompt {
	return Prompt{Role: User, Payload: &Payload{Mime: mime, Uri: uri}}
}

const MimeApplicationPDF = "application/pdf"
const MimeTextPlain = "text/plain"

const MimeAudioMPEG = "audio/mpeg"
const MimeAudioMP3 = "audio/mp3"
const MimeAudioWAV = "audio/wav"

const MimeImagePNG = "image/png"
const MimeImageJPEG = "image/jpeg"
const MimeImageWebp = "image/webp"

const MimeVideoMOV = "video/mov"
const MimeVideoMPEG = "video/mpeg"
const MimeVideoMP4 = "video/mp4"
const MimeVideoMPG = "video/mpg"
const MimeVideoAVI = "video/avi"
const MimeVideoWMV = "video/wmv"
const MimeVideoMPEGS = "video/mpegps"
const MimeVideoFLV = "video/flv"

var MIMEImages map[string]bool = map[string]bool{
	MimeImagePNG:  true,
	MimeImageJPEG: true,
	MimeImageWebp: true,
}
var MIMEAudio map[string]bool = map[string]bool{
	MimeAudioMPEG: true,
	MimeAudioMP3:  true,
	MimeAudioWAV:  true,
}
var MIMEVideo map[string]bool = map[string]bool{
	MimeVideoMOV:   true,
	MimeVideoMPEG:  true,
	MimeVideoMP4:   true,
	MimeVideoMPG:   true,
	MimeVideoAVI:   true,
	MimeVideoWMV:   true,
	MimeVideoMPEGS: true,
	MimeVideoFLV:   true,
}
