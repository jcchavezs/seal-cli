package seal

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
)

// Decode parses lockfile JSON into a Lockfile, rejecting unknown fields and
// trailing bytes. The inbound boundary for every read/validation path -
// stdlib's plain json.Unmarshal silently ignores unknowns, which would let a
// malicious lockfile smuggle data past our checks.
//
// Only enforces JSON constraints (well-formedness, no duplicate object names,
// no unknown fields, no trailing data). Semantic validation (key shape, hash
// format, contentHash recomputation) lives in validate.go.
func Decode(b []byte) (*Lockfile, error) {
	top, err := decodeObject(b)
	if err != nil {
		return nil, fmt.Errorf("decode lockfile: %w", err)
	}
	for key := range top {
		switch key {
		case "version", "discovery", "policy", "bundles":
		default:
			return nil, fmt.Errorf("decode lockfile: unknown field %q", key)
		}
	}

	versionRaw, ok := top["version"]
	if !ok {
		return nil, fmt.Errorf("decode lockfile: version is required")
	}
	var version int
	if err := decodeRequired(versionRaw, "version", &version); err != nil {
		return nil, fmt.Errorf("decode lockfile: %w", err)
	}

	policyRaw, ok := top["policy"]
	if !ok {
		return nil, fmt.Errorf("decode lockfile: policy is required")
	}
	var policy string
	if err := decodeRequired(policyRaw, "policy", &policy); err != nil {
		return nil, fmt.Errorf("decode lockfile: %w", err)
	}

	var discovery []string
	if discoveryRaw, ok := top["discovery"]; ok {
		if err := decodeRequired(discoveryRaw, "discovery", &discovery); err != nil {
			return nil, fmt.Errorf("decode lockfile: %w", err)
		}
	}

	bundlesRaw, ok := top["bundles"]
	if !ok {
		return nil, fmt.Errorf("decode lockfile: bundles is required")
	}
	bundles, err := decodeBundles(bundlesRaw)
	if err != nil {
		return nil, fmt.Errorf("decode lockfile: %w", err)
	}

	return &Lockfile{
		Version:   version,
		Discovery: discovery,
		Policy:    policy,
		Bundles:   bundles,
	}, nil
}

func decodeBundles(raw json.RawMessage) (map[string]Bundle, error) {
	rawBundles, err := decodeRequiredObject(raw, "bundles")
	if err != nil {
		return nil, err
	}
	bundles := make(map[string]Bundle, len(rawBundles))
	for key, rawBundle := range rawBundles {
		bundle, err := decodeBundle(key, rawBundle)
		if err != nil {
			return nil, err
		}
		bundles[key] = bundle
	}
	return bundles, nil
}

func decodeBundle(key string, raw json.RawMessage) (Bundle, error) {
	obj, err := decodeRequiredObject(raw, "bundle "+key)
	if err != nil {
		return Bundle{}, err
	}
	for field := range obj {
		switch field {
		case "revision", "contentHash", "files":
		default:
			return Bundle{}, fmt.Errorf("unknown field %q in bundle %q", field, key)
		}
	}

	contentHashRaw, ok := obj["contentHash"]
	if !ok {
		return Bundle{}, fmt.Errorf("bundle %q: contentHash is required", key)
	}
	var contentHash string
	if err := decodeRequired(contentHashRaw, "contentHash", &contentHash); err != nil {
		return Bundle{}, fmt.Errorf("bundle %q: %w", key, err)
	}

	filesRaw, ok := obj["files"]
	if !ok {
		return Bundle{}, fmt.Errorf("bundle %q: files is required", key)
	}
	files, err := decodeFiles(filesRaw)
	if err != nil {
		return Bundle{}, fmt.Errorf("bundle %q: %w", key, err)
	}

	var revision string
	if revisionRaw, ok := obj["revision"]; ok {
		if err := decodeRequired(revisionRaw, "revision", &revision); err != nil {
			return Bundle{}, fmt.Errorf("bundle %q: %w", key, err)
		}
		if revision == "" {
			return Bundle{}, fmt.Errorf("bundle %q: revision must not be empty", key)
		}
	}

	return Bundle{
		Revision:    revision,
		ContentHash: contentHash,
		Files:       files,
	}, nil
}

func decodeFiles(raw json.RawMessage) (map[string]string, error) {
	rawFiles, err := decodeRequiredObject(raw, "files")
	if err != nil {
		return nil, err
	}
	files := make(map[string]string, len(rawFiles))
	for path, rawHash := range rawFiles {
		var hash string
		if err := decodeRequired(rawHash, "file hash", &hash); err != nil {
			return nil, fmt.Errorf("file %q: %w", path, err)
		}
		files[path] = hash
	}
	return files, nil
}

func decodeRequiredObject(raw json.RawMessage, field string) (map[string]json.RawMessage, error) {
	if isJSONNull(raw) {
		return nil, fmt.Errorf("%s is required", field)
	}
	return decodeObject(raw)
}

func decodeRequired(raw json.RawMessage, field string, out any) error {
	if isJSONNull(raw) {
		return fmt.Errorf("%s is required", field)
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("%s: %w", field, err)
	}
	return nil
}

func isJSONNull(raw json.RawMessage) bool {
	return bytes.Equal(bytes.TrimSpace(raw), []byte("null"))
}

func decodeObject(b []byte) (map[string]json.RawMessage, error) {
	dec := json.NewDecoder(bytes.NewReader(b))

	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	if tok != json.Delim('{') {
		return nil, fmt.Errorf("expected JSON object")
	}

	out := map[string]json.RawMessage{}
	for dec.More() {
		keyTok, err := dec.Token()
		if err != nil {
			return nil, err
		}
		key, ok := keyTok.(string)
		if !ok {
			return nil, fmt.Errorf("expected object key")
		}
		if _, exists := out[key]; exists {
			return nil, fmt.Errorf("duplicate object key %q", key)
		}

		var value json.RawMessage
		if err := dec.Decode(&value); err != nil {
			return nil, err
		}
		out[key] = value
	}

	tok, err = dec.Token()
	if err != nil {
		return nil, err
	}
	if tok != json.Delim('}') {
		return nil, fmt.Errorf("expected end of JSON object")
	}

	var trailing any
	if err := dec.Decode(&trailing); err != io.EOF {
		return nil, fmt.Errorf("unexpected trailing data")
	}
	return out, nil
}
