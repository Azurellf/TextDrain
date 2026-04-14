package asr

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"textdrain/internal/domain"
)

type whisperJSON struct {
	Params struct {
		Model    string `json:"model"`
		Language string `json:"language"`
	} `json:"params"`
	Result struct {
		Language string `json:"language"`
	} `json:"result"`
	Transcription []whisperSegment `json:"transcription"`
}

type whisperSegment struct {
	Timestamps struct {
		From string `json:"from"`
		To   string `json:"to"`
	} `json:"timestamps"`
	Offsets struct {
		From int64 `json:"from"`
		To   int64 `json:"to"`
	} `json:"offsets"`
	Text string `json:"text"`
}

func parseTranscriptFile(path string) (domain.Transcript, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return domain.Transcript{}, fmt.Errorf("%w: %s", ErrMissingTranscript, path)
		}
		return domain.Transcript{}, fmt.Errorf("read whisper transcript %s: %w", path, err)
	}
	return parseTranscriptJSON(data)
}

func parseTranscriptJSON(data []byte) (domain.Transcript, error) {
	var raw whisperJSON
	if err := json.Unmarshal(data, &raw); err != nil {
		return domain.Transcript{}, fmt.Errorf("parse whisper transcript json: %w", err)
	}
	if raw.Transcription == nil {
		return domain.Transcript{}, ErrInvalidTranscript
	}

	segments := make([]domain.TranscriptSegment, 0, len(raw.Transcription))
	textParts := make([]string, 0, len(raw.Transcription))
	for index, segment := range raw.Transcription {
		text := strings.TrimSpace(segment.Text)
		segments = append(segments, domain.TranscriptSegment{
			Index:   index,
			StartMs: segment.Offsets.From,
			EndMs:   segment.Offsets.To,
			Text:    text,
		})
		if text != "" {
			textParts = append(textParts, text)
		}
	}

	language := strings.TrimSpace(raw.Result.Language)
	if language == "" {
		language = strings.TrimSpace(raw.Params.Language)
	}
	if language == "" {
		language = defaultLanguageMode
	}

	metadata := map[string]string{}
	if raw.Params.Model != "" {
		metadata["whisper_params_model"] = raw.Params.Model
	}
	if raw.Params.Language != "" {
		metadata["whisper_params_language"] = raw.Params.Language
	}

	return domain.Transcript{
		Language: language,
		Text:     strings.Join(textParts, "\n"),
		Segments: segments,
		Metadata: metadata,
	}, nil
}
