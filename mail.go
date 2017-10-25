package torus

import (
	"bytes"
	"fmt"
	htmlTemplate "html/template"
	textTemplate "text/template"
	"time"

	"golang.org/x/net/context"
	"google.golang.org/appengine/log"
	"google.golang.org/appengine/mail"
)

const (
	BODY_TEMPLATE = `New questions added in the last 24 hours
========================================

Go to http://go/torus to vote now.

{{range .}}{{.Content}}

Posted: {{.Timestamp}}
Score: {{.Score}}

{{end}}

To unsubscribe, visit http://go/torus and uncheck the check box.`
	HTML_BODY_TEMPLATE = `
<!DOCTYPE html>

<h1>New questions added in the last 24 hours</h1>

<p>Go to <a href="http://town-hall-178521.appspot.com">go/torus</a> to vote now.</p>

{{range .}}
<h2>{{.Content}}</h2>
<div>Posted: {{.Timestamp}}</div>
<div>Score: {{.Score}}</div>
{{end}}

<p>To unsubscribe, visit <a href="http://town-hall-178521.appspot.com">go/torus</a> and uncheck the check box.</p>
`
)

var (
	bodyTemplate     = textTemplate.Must(textTemplate.New("body").Parse(BODY_TEMPLATE))
	htmlBodyTemplate = htmlTemplate.Must(htmlTemplate.New("htmlBody").Parse(HTML_BODY_TEMPLATE))
)

func sendDigests(c context.Context) error {
	_, qs, err := getLatestQuestions(c)
	if err != nil {
		return fmt.Errorf("error getting questions for digest: %v", err)
	}
	if len(qs) <= 0 {
		log.Debugf(c, "no new messages to send")
		return nil
	}

	ss, err := getSubscribers(c)
	if err != nil {
		return fmt.Errorf("error getting subscribers: %v", err)
	}

	subject := fmt.Sprintf("Town Hall Digest %s", time.Now().Format("Mon Jan _2"))
	var bodyBuffer, htmlBodyBuffer bytes.Buffer
	if err := bodyTemplate.Execute(&bodyBuffer, qs); err != nil {
		return fmt.Errorf("error rendering digest: %v", err)
	}
	if err := htmlBodyTemplate.Execute(&htmlBodyBuffer, qs); err != nil {
		return fmt.Errorf("error rendering html digest: %v", err)
	}
	msg := &mail.Message{
		Sender:   "Torus <torus@town-hall-178521.appspotmail.com>",
		Bcc:      ss,
		Subject:  subject,
		Body:     bodyBuffer.String(),
		HTMLBody: htmlBodyBuffer.String(),
	}
	if err := mail.Send(c, msg); err != nil {
		return fmt.Errorf("error sending email digest: %v", err)
	}

	return nil
}
