package schemavalid

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// xmllintBackend shells out to libxml2's command-line validator.
type xmllintBackend struct{ path string }

func (x xmllintBackend) name() string { return "xmllint" }

func (x xmllintBackend) validate(v *Validator, part []byte) error {
	tmp, err := os.CreateTemp("", "spine-part-*.xml")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	if _, err := tmp.Write(part); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}

	cmd := exec.Command(x.path, "--noout", "--nonet", "--schema", v.wrapper, tmp.Name())
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s", firstDiagnostics(stderr.String(), tmp.Name()))
	}
	return nil
}

// lxmlBackend drives the same libxml2 through Python, so a contributor with
// python3-lxml but no libxml2-utils still runs the checks.
type lxmlBackend struct{ path string }

func (l lxmlBackend) name() string { return "python3+lxml" }

const lxmlScript = `
import sys
from lxml import etree
schema = etree.XMLSchema(etree.parse(sys.argv[1]))
doc = etree.parse(sys.argv[2])
if schema.validate(doc):
    sys.exit(0)
for e in list(schema.error_log)[:4]:
    print("%s:%d: %s" % (sys.argv[2], e.line, e.message), file=sys.stderr)
sys.exit(1)
`

func (l lxmlBackend) validate(v *Validator, part []byte) error {
	tmp, err := os.CreateTemp("", "spine-part-*.xml")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	if _, err := tmp.Write(part); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}

	cmd := exec.Command(l.path, "-c", lxmlScript, v.wrapper, tmp.Name())
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s", firstDiagnostics(stderr.String(), tmp.Name()))
	}
	return nil
}

// firstDiagnostics trims a validator's output to the first few messages with
// the temporary file name stripped, so a failure reads as a statement about the
// part rather than about a path that no longer exists.
func firstDiagnostics(out, tmpName string) string {
	var kept []string
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		line = strings.TrimSpace(strings.ReplaceAll(line, tmpName, "part"))
		if line == "" || strings.HasSuffix(line, "fails to validate") {
			continue
		}
		kept = append(kept, line)
		if len(kept) == 3 {
			break
		}
	}
	if len(kept) == 0 {
		return "schema validation failed with no diagnostic"
	}
	return strings.Join(kept, "\n\t")
}
