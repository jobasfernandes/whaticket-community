package linkpreview

import (
	"bytes"
	"context"
	"image"
	"image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
	"golang.org/x/net/html"
)

const (
	pageMaxBytes    = 1 << 20
	imageMaxBytes   = 1 << 20
	maxImageDim     = 4000
	thumbnailWidth  = 100
	thumbnailHeight = 100
	jpegQuality     = 80
	userAgent       = "Mozilla/5.0 (compatible; whaticket-go-worker/1.0; +https://whaticket.com)"
)

var urlRegex = regexp.MustCompile(`(?i)https?://[^\s<>"']+`)

type Preview struct {
	URL           string
	Title         string
	Description   string
	Thumbnail     []byte
	ThumbnailMime string
}

func Extract(ctx context.Context, body string, timeout time.Duration) (*Preview, bool) {
	link := firstURL(body)
	if link == "" {
		return nil, false
	}
	if timeout <= 0 {
		timeout = 5 * time.Second
	}

	fetchCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	pageBytes, err := fetchBytes(fetchCtx, link, pageMaxBytes)
	if err != nil {
		return nil, false
	}

	title, description, imageURL := parseMetadata(pageBytes)
	if title == "" {
		return nil, false
	}

	preview := &Preview{
		URL:         link,
		Title:       title,
		Description: description,
	}

	if imageURL != "" {
		if resolved := resolveImageURL(link, imageURL); resolved != "" {
			if thumb, mime := fetchThumbnail(fetchCtx, resolved); thumb != nil {
				preview.Thumbnail = thumb
				preview.ThumbnailMime = mime
			}
		}
	}

	return preview, true
}

func firstURL(text string) string {
	match := urlRegex.FindString(text)
	if match == "" {
		return ""
	}
	return strings.TrimRight(match, ".,!?;:)]")
}

func fetchBytes(ctx context.Context, target string, maxBytes int64) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,image/*;q=0.8,*/*;q=0.5")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, errStatus(resp.StatusCode)
	}

	limited := io.LimitReader(resp.Body, maxBytes)
	return io.ReadAll(limited)
}

type httpStatusError struct{ code int }

func (e httpStatusError) Error() string { return http.StatusText(e.code) }

func errStatus(code int) error { return httpStatusError{code: code} }

func parseMetadata(pageBytes []byte) (title, description, imageURL string) {
	doc, err := html.Parse(bytes.NewReader(pageBytes))
	if err != nil {
		return "", "", ""
	}

	var (
		ogTitle      string
		twitterTitle string
		fallbackHead string
		ogDesc       string
		metaDesc     string
		ogImage      string
		twitterImage string
		appleIcon    string
		linkIcon     string
	)

	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch n.Data {
			case "title":
				if fallbackHead == "" {
					fallbackHead = collectText(n)
				}
			case "meta":
				prop := strings.ToLower(attr(n, "property"))
				name := strings.ToLower(attr(n, "name"))
				content := attr(n, "content")
				switch {
				case prop == "og:title" && ogTitle == "":
					ogTitle = content
				case (prop == "twitter:title" || name == "twitter:title") && twitterTitle == "":
					twitterTitle = content
				case prop == "og:description" && ogDesc == "":
					ogDesc = content
				case name == "description" && metaDesc == "":
					metaDesc = content
				case prop == "og:image" && ogImage == "":
					ogImage = content
				case (prop == "twitter:image" || name == "twitter:image") && twitterImage == "":
					twitterImage = content
				}
			case "link":
				rel := strings.ToLower(attr(n, "rel"))
				href := attr(n, "href")
				if href != "" {
					if (rel == "apple-touch-icon" || strings.Contains(rel, "apple-touch-icon")) && appleIcon == "" {
						appleIcon = href
					}
					if rel == "icon" && linkIcon == "" {
						linkIcon = href
					}
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)

	title = firstNonEmpty(ogTitle, twitterTitle, strings.TrimSpace(fallbackHead))
	description = firstNonEmpty(ogDesc, metaDesc)
	imageURL = firstNonEmpty(ogImage, twitterImage, appleIcon, linkIcon)
	return
}

func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if strings.EqualFold(a.Key, key) {
			return a.Val
		}
	}
	return ""
}

func collectText(n *html.Node) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.TextNode {
			b.WriteString(node.Data)
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return strings.TrimSpace(b.String())
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if s := strings.TrimSpace(v); s != "" {
			return s
		}
	}
	return ""
}

func resolveImageURL(pageURL, imageURL string) string {
	page, err := url.Parse(pageURL)
	if err != nil {
		return ""
	}
	img, err := url.Parse(imageURL)
	if err != nil {
		return ""
	}
	return page.ResolveReference(img).String()
}

func fetchThumbnail(ctx context.Context, target string) ([]byte, string) {
	data, err := fetchBytes(ctx, target, imageMaxBytes)
	if err != nil || len(data) == 0 {
		return nil, ""
	}

	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, ""
	}
	if cfg.Width > maxImageDim || cfg.Height > maxImageDim {
		return nil, ""
	}

	src, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, ""
	}

	thumb := resizeThumbnail(src)
	var out bytes.Buffer
	if err := jpeg.Encode(&out, thumb, &jpeg.Options{Quality: jpegQuality}); err != nil {
		return nil, ""
	}
	return out.Bytes(), "image/jpeg"
}

func resizeThumbnail(src image.Image) image.Image {
	bounds := src.Bounds()
	srcW := bounds.Dx()
	srcH := bounds.Dy()
	if srcW <= 0 || srcH <= 0 {
		return src
	}

	scaleW := float64(thumbnailWidth) / float64(srcW)
	scaleH := float64(thumbnailHeight) / float64(srcH)
	scale := scaleW
	if scaleH < scale {
		scale = scaleH
	}
	if scale >= 1 {
		return src
	}

	targetW := int(float64(srcW) * scale)
	targetH := int(float64(srcH) * scale)
	if targetW <= 0 {
		targetW = 1
	}
	if targetH <= 0 {
		targetH = 1
	}

	dst := image.NewRGBA(image.Rect(0, 0, targetW, targetH))
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, bounds, draw.Over, nil)
	return dst
}
