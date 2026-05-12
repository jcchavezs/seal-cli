package seal

import (
	"encoding/json"
	"fmt"
	"sort"

	// orderedmap lets us control JSON key order at every nesting level while
	// still delegating value encoding to encoding/json.
	"github.com/iancoleman/orderedmap"
	"golang.org/x/text/unicode/norm"
)

// Encode serialises a Lockfile to deterministic JSON. Two writers on
// different machines must produce byte-identical output for the same logical
// state, which means:
//   - top-level field order is fixed (version, discovery, policy, bundles)
//   - all string keys are NFC-normalised
//   - keys at every level are sorted by raw UTF-8 byte order
//   - 2-space indent, single space after ':', LF line endings, trailing
//     newline
//
// stdlib map encoding sorts but does NOT normalise, so we build our own
// orderedmap to layer normalisation in.
func Encode(lf *Lockfile) ([]byte, error) {
	root := orderedmap.New()
	root.Set("version", lf.Version)
	// Discovery omitted entirely when empty (keeps initial diffs clean).
	if len(lf.Discovery) > 0 {
		root.Set("discovery", sortedNFC(lf.Discovery))
	}
	root.Set("policy", lf.Policy)
	bundles, err := encodeBundles(lf.Bundles)
	if err != nil {
		return nil, err
	}
	root.Set("bundles", bundles)

	out, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return nil, err
	}

	// MarshalIndent omits the trailing newline; append it for POSIX text-file
	// convention.
	return append(out, '\n'), nil
}

// sortedNFC returns a new slice with each entry NFC-normalised and sorted by
// raw UTF-8 byte order. Used for the discovery array.
func sortedNFC(in []string) []string {
	out := make([]string, len(in))
	for i, s := range in {
		out[i] = norm.NFC.String(s)
	}
	sort.Strings(out)
	return out
}

// encodeBundles builds the "bundles" ordered map. We sort the NFC-normalised
// form (not the raw form) because that's what ends up in the output.
func encodeBundles(in map[string]Bundle) (*orderedmap.OrderedMap, error) {
	out := orderedmap.New()
	keys := make([]string, 0, len(in))
	// Remember the original (possibly-NFD) key so we can look up the Bundle
	// value once we've sorted the NFC form.
	original := make(map[string]string, len(in))
	for k := range in {
		nfc := norm.NFC.String(k)
		if prev, ok := original[nfc]; ok {
			return nil, fmt.Errorf("bundle keys %q and %q both NFC-normalize to %q", prev, k, nfc)
		}
		keys = append(keys, nfc)
		original[nfc] = k
	}
	sort.Strings(keys)
	for _, nfcKey := range keys {
		// Routed through encodeBundle so the inner files map also gets NFC
		// normalisation; stdlib JSON would sort but not normalise.
		bundle, err := encodeBundle(in[original[nfcKey]])
		if err != nil {
			return nil, fmt.Errorf("bundle %q: %w", nfcKey, err)
		}
		out.Set(nfcKey, bundle)
	}
	return out, nil
}

// encodeBundle builds the per-bundle ordered map. Revision is omitted when
// empty to keep lockfiles minimal.
func encodeBundle(b Bundle) (*orderedmap.OrderedMap, error) {
	out := orderedmap.New()
	if b.Revision != "" {
		out.Set("revision", b.Revision)
	}
	out.Set("contentHash", b.ContentHash)
	files, err := encodeFiles(b.Files)
	if err != nil {
		return nil, err
	}
	out.Set("files", files)
	return out, nil
}

// encodeFiles builds a bundle's files ordered map; same NFC + sorted rule as
// bundle keys. Hash values are already canonical (sha256:<lowercase-hex>).
func encodeFiles(in map[string]string) (*orderedmap.OrderedMap, error) {
	out := orderedmap.New()
	keys := make([]string, 0, len(in))
	original := make(map[string]string, len(in))
	for k := range in {
		nfc := norm.NFC.String(k)
		if prev, ok := original[nfc]; ok {
			return nil, fmt.Errorf("file paths %q and %q both NFC-normalize to %q", prev, k, nfc)
		}
		keys = append(keys, nfc)
		original[nfc] = k
	}
	sort.Strings(keys)
	for _, nfcKey := range keys {
		out.Set(nfcKey, in[original[nfcKey]])
	}
	return out, nil
}
