package regoref

import (
	"bytes"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

var packageRe = regexp.MustCompile(`(?m)^\s*package\s+([A-Za-z0-9_.]+)`)

type Transform func(filePath string, annot *OrderedMap)

func Run(root string, overwrite bool, out io.Writer, transforms ...Transform) error {
	walkFn := func(filePath string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !strings.HasSuffix(filePath, ".rego") {
			return nil
		}

		data, err := os.ReadFile(filePath)
		if err != nil {
			return err
		}

		if !bytes.HasPrefix(data, []byte("# METADATA")) {
			// skip file without metadata
			return nil
		}

		idx := packageRe.FindSubmatchIndex(data)
		if len(idx) == 0 {
			return nil
		}

		packageStart := idx[0]
		annotationsData := data[:packageStart]

		var buf bytes.Buffer
		lines := bytes.Split(annotationsData, []byte("\n"))
		for i, line := range lines[1:] {
			if i > 0 {
				buf.WriteByte('\n')
			}
			buf.Write(bytes.TrimPrefix(line, []byte("#")))
		}

		var om OrderedMap
		if err := yaml.Unmarshal(buf.Bytes(), &om); err != nil {
			return err
		}

		for _, f := range transforms {
			f(filePath, &om)
		}

		var yamlData bytes.Buffer
		dec := yaml.NewEncoder(&yamlData)
		dec.SetIndent(2)
		defer dec.Close()
		if err := dec.Encode(om); err != nil {
			return err
		}

		var outBuf bytes.Buffer
		outBuf.Grow(len(data))
		outBuf.Write(lines[0])
		outBuf.WriteByte('\n')

		yamlBytes := yamlData.Bytes()
		if yamlBytes[len(yamlBytes)-1] == '\n' {
			yamlBytes = yamlBytes[:len(yamlBytes)-1]
		}

		for line := range bytes.SplitSeq(yamlBytes, []byte("\n")) {
			if len(line) == 0 {
				outBuf.Write([]byte("#\n"))
				continue
			}

			outBuf.Write([]byte("# "))
			outBuf.Write(line)
			outBuf.WriteByte('\n')
		}

		outBuf.Write(data[packageStart:])

		if overwrite {
			fi, err := os.Stat(filePath)
			if err != nil {
				return err
			}
			outfile, err := os.OpenFile(filePath, os.O_WRONLY|os.O_TRUNC, fi.Mode())
			if err != nil {
				return err
			}
			defer outfile.Close()
			out = outfile
		}

		if _, err := out.Write(outBuf.Bytes()); err != nil {
			return err
		}
		return nil
	}

	if err := filepath.WalkDir(root, walkFn); err != nil {
		return err
	}

	return nil
}
