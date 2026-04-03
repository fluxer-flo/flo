package flo

import "time"

type rawSendMessageOpts struct {
	Content *string        `json:"content,omitempty"`
	Embeds  []rawEmbedOpts `json:"embeds,omitempty"`
}

func (m *SendMessageOpts) toRaw() rawSendMessageOpts {
	result := rawSendMessageOpts{Content: &m.Content}

	result.Embeds = make([]rawEmbedOpts, 0, len(m.Embeds))
	for _, embed := range m.Embeds {
		result.Embeds = append(result.Embeds, embed.toRaw())
	}

	return result
}

type rawEmbedOpts struct {
	URL         *string             `json:"url,omitempty"`
	Title       *string             `json:"title,omitempty"`
	Color       *ColorInt           `json:"color,omitempty"`
	Timestamp   *time.Time          `json:"timestamp,omitempty"`
	Description *string             `json:"description,omitempty"`
	Author      *rawEmbedAuthorOpts `json:"author,omitempty"`
	Image       *rawEmbedMediaOpts  `json:"image,omitempty"`
	Thumbnail   *rawEmbedMediaOpts  `json:"thumbnail,omitempty"`
	Footer      *rawEmbedFooterOpts `json:"footer,omitempty"`
	Fields      []rawEmbedField     `json:"fields,omitempty"`
}

func (e *EmbedOpts) toRaw() rawEmbedOpts {
	var result rawEmbedOpts

	if e.URL != "" {
		result.URL = &e.URL
	}

	if e.Title != "" {
		result.Title = &e.Title
	}

	result.Color = &e.Color

	if !e.Timestamp.IsZero() {
		result.Timestamp = &e.Timestamp
	}

	if e.Description != "" {
		result.Description = &e.Description
	}

	if e.Author != (EmbedAuthorOpts{}) {
		a := e.Author.toRaw()
		result.Author = &a
	}

	if e.Image != (EmbedMediaOpts{}) {
		m := e.Image.toRaw()
		result.Image = &m
	}

	if e.Thumbnail != (EmbedMediaOpts{}) {
		m := e.Thumbnail.toRaw()
		result.Thumbnail = &m
	}

	if e.Footer != (EmbedFooterOpts{}) {
		f := e.Footer.toRaw()
		result.Footer = &f
	}

	result.Fields = make([]rawEmbedField, 0, len(e.Fields))
	for _, field := range e.Fields {
		result.Fields = append(result.Fields, rawEmbedField(field))
	}

	return result
}

type rawEmbedAuthorOpts struct {
	Name    string  `json:"name"`
	URL     *string `json:"url,omitempty"`
	IconURL *string `json:"icon_url,omitempty"`
}

func (a EmbedAuthorOpts) toRaw() rawEmbedAuthorOpts {
	result := rawEmbedAuthorOpts{Name: a.Name}

	if a.URL != "" {
		result.URL = &a.URL
	}

	if a.IconURL != "" {
		result.IconURL = &a.IconURL
	}

	return result
}

type rawEmbedMediaOpts struct {
	URL         string  `json:"url"`
	Description *string `json:"description,omitempty"`
}

func (m EmbedMediaOpts) toRaw() rawEmbedMediaOpts {
	result := rawEmbedMediaOpts{URL: m.URL}

	if m.Description != "" {
		result.Description = &m.Description
	}

	return result
}

type rawEmbedFooterOpts struct {
	Text    string  `json:"text"`
	IconURL *string `json:"icon_url,omitempty"`
}

func (f EmbedFooterOpts) toRaw() rawEmbedFooterOpts {
	result := rawEmbedFooterOpts{Text: f.Text}

	if result.IconURL != nil {
		result.IconURL = &f.IconURL
	}

	return result
}
