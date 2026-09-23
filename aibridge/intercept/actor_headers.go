package intercept

import (
	"fmt"
	"strings"

	"github.com/coder/coder/v2/aibridge/context"
)

const (
	prefix = "X-AI-Bridge-Actor"
)

func ActorIDHeader() string {
	return fmt.Sprintf("%s-ID", prefix)
}

func ActorMetadataHeader(name string) string {
	return fmt.Sprintf("%s-Metadata-%s", prefix, name)
}

func IsActorHeader(name string) bool {
	return strings.HasPrefix(strings.ToLower(name), strings.ToLower(prefix))
}

func resolvedActorHeaderNames(names map[string]string) map[string]string {
	resolved := map[string]string{
		"id":       ActorIDHeader(),
		"username": ActorMetadataHeader("Username"),
		"email":    ActorMetadataHeader("Email"),
	}
	for key, name := range names {
		if _, ok := resolved[key]; ok && name != "" {
			resolved[key] = name
		}
	}
	return resolved
}

func headersFromActor(actor *context.Actor, names map[string]string) map[string]string {
	if actor == nil {
		return nil
	}

	resolved := resolvedActorHeaderNames(names)
	headers := make(map[string]string, len(actor.Metadata)+1)
	for k, v := range actor.Metadata {
		if k == "Username" || k == "Email" {
			continue
		}
		headers[ActorMetadataHeader(k)] = fmt.Sprintf("%v", v)
	}

	headers[resolved["id"]] = actor.ID
	for _, key := range []string{"Username", "Email"} {
		if value, ok := actor.Metadata[key]; ok {
			if value := fmt.Sprintf("%v", value); value != "" {
				headers[resolved[strings.ToLower(key)]] = value
			}
		}
	}
	return headers
}
