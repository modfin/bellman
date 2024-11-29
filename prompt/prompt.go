package prompt

import "io"

type Role string

const User = Role("user")
const Assistant = Role("assistant")

type Prompt struct {
	Role    Role
	Text    string
	Payload *Payload
}

type Payload struct {
	Mime string
	Data io.Reader
}

func AsAssistant(text string) Prompt {
	return Prompt{Role: Assistant, Text: text}
}
func AsUser(text string) Prompt {
	return Prompt{Role: User, Text: text}
}
func AsUserWithData(mime string, reader io.Reader) Prompt {
	return Prompt{Role: User, Payload: &Payload{Mime: mime, Data: reader}}
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
