package parser

import (
	"archive/zip"
	"encoding/xml"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"golang.org/x/net/html"
)

type epubContainer struct {
	Rootfile struct {
		FullPath string `xml:"full-path,attr"`
	} `xml:"rootfiles>rootfile"`
}

type epubPackage struct {
	Metadata struct {
		Title  string `xml:"title"`
		Author string `xml:"creator"`
	} `xml:"metadata"`
	Manifest struct {
		Items []struct {
			ID       string `xml:"id,attr"`
			Href     string `xml:"href,attr"`
		} `xml:"item"`
	} `xml:"manifest"`
	Spine struct {
		Items []struct {
			IDRef string `xml:"idref,attr"`
		} `xml:"itemref"`
	} `xml:"spine"`
}

func ParseEPUB(path string) (*Book, error) {
	r, err := zip.OpenReader(path)
	if err != nil {
		return nil, fmt.Errorf("open epub: %w", err)
	}
	defer r.Close()

	containerData, err := readZipEntry(r, "META-INF/container.xml")
	if err != nil {
		return nil, fmt.Errorf("container.xml: %w", err)
	}

	var c epubContainer
	if err := xml.Unmarshal(containerData, &c); err != nil {
		return nil, fmt.Errorf("parse container.xml: %w", err)
	}

	opfPath := c.Rootfile.FullPath
	opfDir := filepath.Dir(opfPath)
	if opfDir == "." {
		opfDir = ""
	}

	opfData, err := readZipEntry(r, opfPath)
	if err != nil {
		return nil, fmt.Errorf("opf: %w", err)
	}

	var pkg epubPackage
	if err := xml.Unmarshal(opfData, &pkg); err != nil {
		return nil, fmt.Errorf("parse opf: %w", err)
	}

	idToHref := make(map[string]string, len(pkg.Manifest.Items))
	for _, item := range pkg.Manifest.Items {
		href := item.Href
		if opfDir != "" {
			href = opfDir + "/" + href
		}
		idToHref[item.ID] = href
	}

	var sb strings.Builder
	for _, ref := range pkg.Spine.Items {
		href, ok := idToHref[ref.IDRef]
		if !ok {
			continue
		}
		data, err := readZipEntry(r, href)
		if err != nil {
			continue
		}
		sb.WriteString(htmlToText(data))
		sb.WriteByte('\n')
	}

	return &Book{
		Title:   pkg.Metadata.Title,
		Author:  pkg.Metadata.Author,
		Content: sb.String(),
	}, nil
}

func readZipEntry(r *zip.ReadCloser, name string) ([]byte, error) {
	for _, f := range r.File {
		if f.Name == name {
			rc, err := f.Open()
			if err != nil {
				return nil, err
			}
			defer rc.Close()
			return io.ReadAll(rc)
		}
	}
	return nil, fmt.Errorf("not found in epub: %s", name)
}

func htmlToText(data []byte) string {
	doc, err := html.Parse(strings.NewReader(string(data)))
	if err != nil {
		return ""
	}
	var sb strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch n.Data {
			case "script", "style", "head":
				return
			}
		}
		if n.Type == html.TextNode {
			if t := strings.TrimSpace(n.Data); t != "" {
				sb.WriteString(t)
				sb.WriteByte(' ')
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return strings.TrimSpace(sb.String())
}
