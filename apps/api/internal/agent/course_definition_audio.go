package agent

// course_definition_audio.go — publish-time TTS pre-generation for
// CourseDefinition 2.0 courses (Course Authoring & Publish Lifecycle, Task 5).
// GenerateDefinitionAudio walks a stored 2.0 definition's narrations plus the
// opening/closing fallback, synthesizes one mp3 per authored audio path, and
// uploads it to OSS at EXACTLY the path the document references — unlike
// GenerateCourseAudio (course_audio.go), which derives its own hashed object
// key for legacy render_cache courses, a 2.0 definition already names its own
// audio path per narration, so this function is a straight synth+put, no
// idempotency/existence check and no manifest.

import (
	"context"
	"encoding/json"
	"fmt"
)

// definitionAudioDoc is the border slice of a CourseDefinition 2.0 needed to
// generate narration audio: every narration's text+audio path, plus the
// opening/closing fallback audio. Deep structure (interactions, gates,
// objectives, ...) is irrelevant here and deliberately not parsed.
type definitionAudioDoc struct {
	Course struct {
		Opening struct {
			Fallback struct{ Text, Audio string } `json:"fallback"`
		} `json:"opening"`
		Closing struct {
			Fallback struct{ Text, Audio string } `json:"fallback"`
		} `json:"closing"`
		Parts []struct {
			Slices []struct {
				Narrations []struct {
					Text  string `json:"text"`
					Audio string `json:"audio"`
				} `json:"narrations"`
			} `json:"slices"`
		} `json:"parts"`
	} `json:"course"`
}

// GenerateDefinitionAudio synthesizes TTS for each narration (and the
// opening/closing fallback when it has an audio path) in a CourseDefinition
// 2.0 document and uploads the mp3 to courses/<slug>/<authored audio path> —
// the exact path the document references. Returns the number of clips
// generated. synth/store nil (voice/OSS unconfigured) degrades to (0, nil):
// ship still flips status, just without audio. A clip whose text or audio
// path is empty is skipped, not an error.
func GenerateDefinitionAudio(ctx context.Context, synth CourseAudioSynth, store CourseAudioStore, slug string, definition []byte) (int, error) {
	if synth == nil || store == nil {
		return 0, nil
	}
	var doc definitionAudioDoc
	if err := json.Unmarshal(definition, &doc); err != nil {
		return 0, fmt.Errorf("definition audio: parse: %w", err)
	}

	type clip struct{ text, audio string }
	var clips []clip
	add := func(text, audio string) {
		if text != "" && audio != "" {
			clips = append(clips, clip{text, audio})
		}
	}
	add(doc.Course.Opening.Fallback.Text, doc.Course.Opening.Fallback.Audio)
	add(doc.Course.Closing.Fallback.Text, doc.Course.Closing.Fallback.Audio)
	for _, p := range doc.Course.Parts {
		for _, s := range p.Slices {
			for _, n := range s.Narrations {
				add(n.Text, n.Audio)
			}
		}
	}

	n := 0
	for _, c := range clips {
		key := "courses/" + slug + "/" + c.audio
		audio, err := synth.Synthesize(ctx, c.text, synthSpeed)
		if err != nil {
			return n, fmt.Errorf("definition audio: synth %q: %w", c.audio, err)
		}
		if err := store.PutObject(ctx, key, "audio/mpeg", audio); err != nil {
			return n, fmt.Errorf("definition audio: put %q: %w", key, err)
		}
		n++
	}
	return n, nil
}
