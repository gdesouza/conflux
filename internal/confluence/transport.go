package confluence

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

const maxErrorBodyBytes = 64 << 10

type APIError struct {
	StatusCode int
	Method     string
	URL        string
	Body       string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("confluence API %s %s failed with status %d: %s", e.Method, e.URL, e.StatusCode, e.Body)
}

func IsNotFound(err error) bool {
	var apiError *APIError
	return errors.As(err, &apiError) && apiError.StatusCode == http.StatusNotFound
}

func IsVersionConflict(err error) bool {
	var apiError *APIError
	return errors.As(err, &apiError) && apiError.StatusCode == http.StatusConflict
}

func (c *Client) doAuthenticated(req *http.Request) (*http.Response, error) {
	if err := c.validateOrigin(req.URL); err != nil {
		return nil, err
	}
	req.SetBasicAuth(c.username, c.apiToken)
	resp, err := c.client.Do(req) // #nosec G704 -- validateOrigin restricts requests to the configured Confluence origin.
	if err != nil {
		return nil, fmt.Errorf("execute Confluence request: %w", err)
	}
	return resp, nil
}

func responseError(resp *http.Response) error {
	body, readErr := io.ReadAll(io.LimitReader(resp.Body, maxErrorBodyBytes))
	if readErr != nil {
		return fmt.Errorf("read Confluence error response: %w", readErr)
	}
	return &APIError{
		StatusCode: resp.StatusCode,
		Method:     resp.Request.Method,
		URL:        resp.Request.URL.String(),
		Body:       strings.TrimSpace(string(body)),
	}
}

func (c *Client) DownloadAttachment(ctx context.Context, pageID, attachmentID string) (io.ReadCloser, error) {
	attachments, err := c.ListAttachmentsContext(ctx, pageID)
	if err != nil {
		return nil, err
	}
	var downloadReference string
	for _, attachment := range attachments {
		if attachment.ID != attachmentID {
			continue
		}
		downloadReference = attachment.Links.Download
		if downloadReference == "" {
			downloadReference = attachment.Download
		}
		break
	}
	if downloadReference == "" {
		return nil, fmt.Errorf("attachment %s not found for page %s", attachmentID, pageID)
	}
	downloadURL, err := c.resolveURL(downloadReference)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create attachment download request: %w", err)
	}
	resp, err := c.doAuthenticated(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		defer resp.Body.Close()
		return nil, responseError(resp)
	}
	return resp.Body, nil
}

func (c *Client) resolveURL(reference string) (string, error) {
	base, err := url.Parse(c.baseURL)
	if err != nil {
		return "", fmt.Errorf("parse Confluence base URL: %w", err)
	}
	if strings.HasPrefix(reference, "/") {
		return strings.TrimRight(c.baseURL, "/") + reference, nil
	}
	next, err := url.Parse(reference)
	if err != nil {
		return "", fmt.Errorf("parse Confluence response URL: %w", err)
	}
	resolved := base.ResolveReference(next)
	if err := c.validateOrigin(resolved); err != nil {
		return "", err
	}
	return resolved.String(), nil
}

func (c *Client) validateOrigin(requestURL *url.URL) error {
	base, err := url.Parse(c.baseURL)
	if err != nil {
		return fmt.Errorf("parse Confluence base URL: %w", err)
	}
	if !strings.EqualFold(requestURL.Scheme, base.Scheme) || !strings.EqualFold(requestURL.Host, base.Host) {
		return fmt.Errorf("refuse Confluence request to untrusted origin %q", requestURL.Host)
	}
	return nil
}
