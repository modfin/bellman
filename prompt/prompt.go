package prompt

type Role string

const User = Role("user")
const Assistant = Role("assistant")

type Prompt struct {
	Role Role
	Text string
}

func AsAssistant(text string) Prompt {
	return Prompt{Role: Assistant, Text: text}
}
func AsUser(text string) Prompt {
	return Prompt{Role: User, Text: text}
}
