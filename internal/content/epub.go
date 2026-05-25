package content

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"html"
	"io"
	"net/url"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode"

	xhtml "golang.org/x/net/html"
)

type Book struct {
	Title          string
	Author         string
	Chapters       []Chapter
	CoverPath      string
	CoverMediaType string
}

type Chapter struct {
	ID    string
	Title string
	Href  string
	Text  string
}

type Cover struct {
	MediaType string
	Data      []byte
}

func ParseEPUB(filePath string) (*Book, error) {
	reader, err := zip.OpenReader(filePath)
	if err != nil {
		return nil, fmt.Errorf("open epub: %w", err)
	}
	defer reader.Close()

	files := mapZipFiles(reader.File)
	book, err := parseWithPackage(files)
	if err != nil {
		return nil, err
	}
	if book == nil {
		book = &Book{}
	}
	if len(book.Chapters) == 0 {
		book.Chapters = fallbackChapters(files)
	}
	if book.Title == "" {
		book.Title = strings.TrimSuffix(filepath.Base(filePath), filepath.Ext(filePath))
	}
	return book, nil
}

type containerXML struct {
	Rootfiles []struct {
		FullPath string `xml:"full-path,attr"`
	} `xml:"rootfiles>rootfile"`
}

type packageXML struct {
	Metadata struct {
		Title   string `xml:"title"`
		Creator string `xml:"creator"`
		Meta    []struct {
			Name    string `xml:"name,attr"`
			Content string `xml:"content,attr"`
		} `xml:"meta"`
	} `xml:"metadata"`
	Manifest []opfItem `xml:"manifest>item"`
	Spine    []struct {
		IDRef string `xml:"idref,attr"`
	} `xml:"spine>itemref"`
}

type opfItem struct {
	ID         string `xml:"id,attr"`
	Href       string `xml:"href,attr"`
	MediaType  string `xml:"media-type,attr"`
	Properties string `xml:"properties,attr"`
}

func parseWithPackage(files map[string]*zip.File) (*Book, error) {
	opfPath, err := packagePath(files)
	if err != nil {
		return nil, err
	}
	if opfPath == "" {
		return nil, nil
	}

	body, err := readZipFile(files[opfPath])
	if err != nil {
		return nil, err
	}

	var pkg packageXML
	if err := xml.Unmarshal(body, &pkg); err != nil {
		return nil, fmt.Errorf("parse opf: %w", err)
	}

	base := path.Dir(opfPath)
	itemsByID := make(map[string]opfItem, len(pkg.Manifest))
	itemsByEntry := make(map[string]opfItem, len(pkg.Manifest))
	for _, item := range pkg.Manifest {
		itemsByID[item.ID] = item
		itemsByEntry[entryPath(base, item.Href)] = item
	}

	book := &Book{
		Title:  strings.TrimSpace(pkg.Metadata.Title),
		Author: strings.TrimSpace(pkg.Metadata.Creator),
	}
	book.CoverPath, book.CoverMediaType = findCoverEntry(base, pkg, itemsByID)
	for _, ref := range pkg.Spine {
		item, ok := itemsByID[ref.IDRef]
		if !ok || !isReadableItem(item.Href, item.MediaType) {
			continue
		}
		entry := entryPath(base, item.Href)
		chapter, err := chapterFromEntry(files, entry, item.ID)
		if err != nil {
			return nil, err
		}
		if chapter != nil {
			book.Chapters = append(book.Chapters, *chapter)
		}
	}
	if len(book.Chapters) == 0 {
		for _, chapterRef := range navChapters(files, base, pkg.Manifest, itemsByEntry) {
			chapter, err := chapterFromEntryWithTitle(files, chapterRef.Entry, chapterRef.ID, chapterRef.Title)
			if err != nil {
				return nil, err
			}
			if chapter != nil {
				book.Chapters = append(book.Chapters, *chapter)
			}
		}
	}
	return book, nil
}

func ExtractEPUBCover(filePath string) (*Cover, error) {
	reader, err := zip.OpenReader(filePath)
	if err != nil {
		return nil, fmt.Errorf("open epub: %w", err)
	}
	defer reader.Close()

	files := mapZipFiles(reader.File)
	entry, mediaType, err := coverEntry(files)
	if err != nil {
		return nil, err
	}
	if entry == "" {
		return nil, nil
	}
	body, err := readZipFile(files[entry])
	if err != nil {
		return nil, err
	}
	return &Cover{MediaType: mediaType, Data: body}, nil
}

func entryPath(base, href string) string {
	if base == "." {
		base = ""
	}
	href = strings.Split(href, "#")[0]
	return cleanArchivePath(path.Join(base, href))
}

func packagePath(files map[string]*zip.File) (string, error) {
	containerFile := files["META-INF/container.xml"]
	if containerFile == nil {
		for name := range files {
			if strings.EqualFold(name, "META-INF/container.xml") {
				containerFile = files[name]
				break
			}
		}
	}
	if containerFile != nil {
		body, err := readZipFile(containerFile)
		if err != nil {
			return "", err
		}
		var container containerXML
		if err := xml.Unmarshal(body, &container); err != nil {
			return "", fmt.Errorf("parse container.xml: %w", err)
		}
		for _, rootfile := range container.Rootfiles {
			if rootfile.FullPath != "" {
				return cleanArchivePath(rootfile.FullPath), nil
			}
		}
	}

	var candidates []string
	for name := range files {
		if strings.EqualFold(path.Ext(name), ".opf") {
			candidates = append(candidates, name)
		}
	}
	sort.Strings(candidates)
	if len(candidates) == 0 {
		return "", nil
	}
	return candidates[0], nil
}

func fallbackChapters(files map[string]*zip.File) []Chapter {
	var names []string
	for name := range files {
		if isReadablePath(name) {
			names = append(names, name)
		}
	}
	sort.Strings(names)

	chapters := make([]Chapter, 0, len(names))
	for _, name := range names {
		chapter, err := chapterFromEntry(files, name, "")
		if err == nil && chapter != nil {
			chapters = append(chapters, *chapter)
		}
	}
	return chapters
}

func chapterFromEntry(files map[string]*zip.File, entry, id string) (*Chapter, error) {
	return chapterFromEntryWithTitle(files, entry, id, "")
}

func chapterFromEntryWithTitle(files map[string]*zip.File, entry, id, title string) (*Chapter, error) {
	file := files[entry]
	if file == nil {
		return nil, nil
	}
	body, err := readZipFile(file)
	if err != nil {
		return nil, err
	}
	text := readableText(body)
	if text == "" {
		return nil, nil
	}
	if title == "" {
		title = firstHeading(body)
		if title == "" {
			title = strings.TrimSuffix(path.Base(entry), path.Ext(entry))
		}
	}
	if id == "" {
		id = IDFromPath(entry)
	}
	return &Chapter{ID: id, Title: title, Href: entry, Text: text}, nil
}

type navChapter struct {
	ID    string
	Entry string
	Title string
}

func navChapters(files map[string]*zip.File, base string, manifest []opfItem, itemsByEntry map[string]opfItem) []navChapter {
	var navItems []opfItem
	for _, item := range manifest {
		if strings.Contains(strings.ToLower(item.Properties), "nav") || strings.EqualFold(path.Ext(item.Href), ".ncx") || strings.Contains(strings.ToLower(item.MediaType), "ncx") {
			navItems = append(navItems, item)
		}
	}

	var chapters []navChapter
	seen := map[string]bool{}
	for _, item := range navItems {
		entry := entryPath(base, item.Href)
		file := files[entry]
		if file == nil {
			continue
		}
		body, err := readZipFile(file)
		if err != nil {
			continue
		}
		links := parseNavLinks(body)
		if isNCXItem(item) {
			links = parseNCXLinks(body)
		}
		for _, link := range links {
			chapterEntry := entryPath(path.Dir(entry), link.Href)
			if seen[chapterEntry] || !isReadablePath(chapterEntry) {
				continue
			}
			seen[chapterEntry] = true
			id := ""
			if manifestItem, ok := itemsByEntry[chapterEntry]; ok {
				id = manifestItem.ID
			}
			chapters = append(chapters, navChapter{
				ID:    id,
				Entry: chapterEntry,
				Title: link.Title,
			})
		}
	}
	return chapters
}

func coverEntry(files map[string]*zip.File) (string, string, error) {
	opfPath, err := packagePath(files)
	if err != nil {
		return "", "", err
	}
	if opfPath != "" {
		body, err := readZipFile(files[opfPath])
		if err != nil {
			return "", "", err
		}

		var pkg packageXML
		if err := xml.Unmarshal(body, &pkg); err != nil {
			return "", "", fmt.Errorf("parse opf: %w", err)
		}
		itemsByID := make(map[string]opfItem, len(pkg.Manifest))
		for _, item := range pkg.Manifest {
			itemsByID[item.ID] = item
		}
		if entry, mediaType := findCoverEntry(path.Dir(opfPath), pkg, itemsByID); entry != "" {
			return entry, mediaType, nil
		}
	}

	return fallbackCoverEntry(files), fallbackCoverMediaType(files), nil
}

func findCoverEntry(base string, pkg packageXML, itemsByID map[string]opfItem) (string, string) {
	for _, meta := range pkg.Metadata.Meta {
		if !strings.EqualFold(strings.TrimSpace(meta.Name), "cover") {
			continue
		}
		if item, ok := itemsByID[strings.TrimSpace(meta.Content)]; ok && isImageItem(item) {
			return entryPath(base, item.Href), mediaTypeForItem(item)
		}
	}
	for _, item := range pkg.Manifest {
		if strings.Contains(strings.ToLower(item.Properties), "cover-image") && isImageItem(item) {
			return entryPath(base, item.Href), mediaTypeForItem(item)
		}
	}
	for _, item := range pkg.Manifest {
		lowerHref := strings.ToLower(item.Href)
		lowerID := strings.ToLower(item.ID)
		if isImageItem(item) && (strings.Contains(lowerHref, "cover") || strings.Contains(lowerID, "cover")) {
			return entryPath(base, item.Href), mediaTypeForItem(item)
		}
	}
	return "", ""
}

func fallbackCoverEntry(files map[string]*zip.File) string {
	var fallback string
	for name := range files {
		if !isImagePath(name) {
			continue
		}
		lower := strings.ToLower(name)
		if strings.Contains(lower, "cover") {
			return name
		}
		if fallback == "" {
			fallback = name
		}
	}
	return fallback
}

func fallbackCoverMediaType(files map[string]*zip.File) string {
	entry := fallbackCoverEntry(files)
	if entry == "" {
		return ""
	}
	switch strings.ToLower(path.Ext(entry)) {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".webp":
		return "image/webp"
	case ".gif":
		return "image/gif"
	default:
		return "application/octet-stream"
	}
}

func isImageItem(item opfItem) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(item.MediaType)), "image/") || isImagePath(item.Href)
}

func isImagePath(name string) bool {
	switch strings.ToLower(path.Ext(name)) {
	case ".jpg", ".jpeg", ".png", ".webp", ".gif":
		return true
	default:
		return false
	}
}

func mediaTypeForItem(item opfItem) string {
	value := strings.TrimSpace(strings.ToLower(item.MediaType))
	if value != "" {
		return value
	}
	switch strings.ToLower(path.Ext(item.Href)) {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".webp":
		return "image/webp"
	case ".gif":
		return "image/gif"
	default:
		return "application/octet-stream"
	}
}

func isNCXItem(item opfItem) bool {
	return strings.EqualFold(path.Ext(item.Href), ".ncx") || strings.Contains(strings.ToLower(item.MediaType), "ncx")
}

type navLink struct {
	Href  string
	Title string
}

type ncxXML struct {
	Points []ncxPoint `xml:"navMap>navPoint"`
}

type ncxPoint struct {
	Label struct {
		Text string `xml:"text"`
	} `xml:"navLabel"`
	Content struct {
		Src string `xml:"src,attr"`
	} `xml:"content"`
	Children []ncxPoint `xml:"navPoint"`
}

func parseNCXLinks(body []byte) []navLink {
	var ncx ncxXML
	if err := xml.Unmarshal(body, &ncx); err != nil {
		return nil
	}
	var links []navLink
	var walk func([]ncxPoint)
	walk = func(points []ncxPoint) {
		for _, point := range points {
			if point.Content.Src != "" {
				links = append(links, navLink{
					Href:  point.Content.Src,
					Title: normalizeSpaces(point.Label.Text),
				})
			}
			walk(point.Children)
		}
	}
	walk(ncx.Points)
	return links
}

func parseNavLinks(body []byte) []navLink {
	tokenizer := xhtml.NewTokenizer(bytes.NewReader(body))
	var links []navLink
	var current *navLink
	var text strings.Builder

	for {
		switch tokenizer.Next() {
		case xhtml.ErrorToken:
			return links
		case xhtml.StartTagToken:
			name, hasAttr := tokenizer.TagName()
			if string(name) != "a" && string(name) != "content" {
				continue
			}
			for hasAttr {
				key, value, more := tokenizer.TagAttr()
				attr := string(key)
				if attr == "href" || attr == "src" {
					current = &navLink{Href: string(value)}
					text.Reset()
					break
				}
				hasAttr = more
			}
		case xhtml.TextToken:
			if current != nil {
				text.Write(tokenizer.Text())
			}
		case xhtml.EndTagToken:
			name, _ := tokenizer.TagName()
			if current != nil && (string(name) == "a" || string(name) == "content") {
				current.Title = normalizeSpaces(html.UnescapeString(text.String()))
				if current.Href != "" {
					links = append(links, *current)
				}
				current = nil
			}
		}
	}
}

func mapZipFiles(files []*zip.File) map[string]*zip.File {
	out := make(map[string]*zip.File, len(files))
	for _, file := range files {
		out[cleanArchivePath(file.Name)] = file
	}
	return out
}

func readZipFile(file *zip.File) ([]byte, error) {
	rc, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	body, err := io.ReadAll(rc)
	if err != nil {
		return nil, err
	}
	return body, nil
}

func isReadableItem(href, mediaType string) bool {
	mediaType = strings.ToLower(mediaType)
	return strings.Contains(mediaType, "html") || strings.Contains(mediaType, "text") || isReadablePath(href)
}

func isReadablePath(name string) bool {
	ext := strings.ToLower(path.Ext(name))
	return ext == ".xhtml" || ext == ".html" || ext == ".htm" || ext == ".txt"
}

func cleanArchivePath(name string) string {
	name = strings.TrimSpace(name)
	if decoded, err := url.PathUnescape(name); err == nil {
		name = decoded
	}
	return path.Clean(strings.TrimPrefix(strings.ReplaceAll(name, "\\", "/"), "/"))
}

func IDFromPath(name string) string {
	return slugPath(strings.Trim(name, "/"))
}

func readableText(body []byte) string {
	tokenizer := xhtml.NewTokenizer(bytes.NewReader(body))
	var b strings.Builder
	skipDepth := 0

	for {
		tokenType := tokenizer.Next()
		switch tokenType {
		case xhtml.ErrorToken:
			if errors.Is(tokenizer.Err(), io.EOF) {
				return normalizeText(b.String())
			}
			return normalizeText(b.String())
		case xhtml.TextToken:
			if skipDepth > 0 {
				continue
			}
			text := strings.TrimSpace(html.UnescapeString(string(tokenizer.Text())))
			if text != "" {
				appendText(&b, text)
			}
		case xhtml.StartTagToken:
			name, _ := tokenizer.TagName()
			tag := string(name)
			if isSkippedTag(tag) {
				skipDepth++
				continue
			}
			if tag == "br" {
				appendNewline(&b, 1)
			}
			if isBlockTag(tag) {
				appendNewline(&b, 2)
			}
		case xhtml.SelfClosingTagToken:
			name, _ := tokenizer.TagName()
			tag := string(name)
			if tag == "br" {
				appendNewline(&b, 1)
			}
		case xhtml.EndTagToken:
			name, _ := tokenizer.TagName()
			tag := string(name)
			if skipDepth > 0 {
				if isSkippedTag(tag) {
					skipDepth--
				}
				continue
			}
			if isBlockTag(tag) {
				appendNewline(&b, 2)
			}
		}
	}
}

func firstHeading(body []byte) string {
	tokenizer := xhtml.NewTokenizer(bytes.NewReader(body))
	var capture string
	var b strings.Builder

	for {
		tokenType := tokenizer.Next()
		switch tokenType {
		case xhtml.ErrorToken:
			return normalizeSpaces(b.String())
		case xhtml.StartTagToken:
			name, _ := tokenizer.TagName()
			tag := string(name)
			if tag == "h1" || tag == "h2" || tag == "h3" || tag == "h4" || tag == "h5" || tag == "h6" {
				capture = tag
				b.Reset()
			}
		case xhtml.TextToken:
			if capture != "" {
				b.WriteString(html.UnescapeString(string(tokenizer.Text())))
			}
		case xhtml.EndTagToken:
			name, _ := tokenizer.TagName()
			if capture != "" && string(name) == capture {
				title := normalizeSpaces(b.String())
				if title != "" {
					return title
				}
				capture = ""
			}
		}
	}
}

func appendText(b *strings.Builder, text string) {
	if b.Len() > 0 {
		last := b.String()[b.Len()-1]
		if last != '\n' && last != ' ' {
			b.WriteByte(' ')
		}
	}
	b.WriteString(normalizeSpaces(text))
}

func appendNewline(b *strings.Builder, count int) {
	if b.Len() == 0 {
		return
	}
	current := 0
	s := b.String()
	for i := len(s) - 1; i >= 0 && s[i] == '\n'; i-- {
		current++
	}
	for current < count {
		b.WriteByte('\n')
		current++
	}
}

func isBlockTag(tag string) bool {
	switch tag {
	case "address", "article", "aside", "blockquote", "body", "div", "footer", "h1", "h2", "h3", "h4", "h5", "h6", "header", "li", "main", "nav", "ol", "p", "pre", "section", "table", "tr", "ul":
		return true
	default:
		return false
	}
}

func isSkippedTag(tag string) bool {
	switch tag {
	case "head", "title", "script", "style", "metadata":
		return true
	default:
		return false
	}
}

var (
	spaceRe   = regexp.MustCompile(`[ \t\r\f\v]+`)
	newlineRe = regexp.MustCompile(`\n{3,}`)
)

func normalizeText(text string) string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lines[i] = normalizeSpaces(line)
	}
	text = strings.Join(lines, "\n")
	text = newlineRe.ReplaceAllString(text, "\n\n")
	return strings.TrimSpace(text)
}

func normalizeSpaces(text string) string {
	text = strings.ReplaceAll(text, "\u00a0", " ")
	return strings.TrimSpace(spaceRe.ReplaceAllString(text, " "))
}

func slug(value string) string {
	value = strings.TrimSuffix(value, path.Ext(value))
	return slugPath(value)
}

func slugPath(value string) string {
	var b strings.Builder
	lastDash := false
	for _, r := range strings.ToLower(value) {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash && b.Len() > 0 {
			b.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}
