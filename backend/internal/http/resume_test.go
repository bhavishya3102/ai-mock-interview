package http

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/bhavishya3102/ai-mock-interview/backend/internal/domain"
	"github.com/stretchr/testify/require"
)

// resumeMultipart builds a multipart body with content in the named field.
func resumeMultipart(t *testing.T, field string, content []byte) (*bytes.Buffer, string) {
	t.Helper()
	body := &bytes.Buffer{}
	w := multipart.NewWriter(body)
	part, err := w.CreateFormFile(field, "resume.pdf")
	require.NoError(t, err)
	_, err = part.Write(content)
	require.NoError(t, err)
	require.NoError(t, w.Close())
	return body, w.FormDataContentType()
}

func TestUploadResume_Unauthorized(t *testing.T) {
	body, ct := resumeMultipart(t, "resume", []byte("%PDF-1.7 content"))
	h := NewInterviewHandler(&fakeSvc{}, discardLogger())
	req := httptest.NewRequest(http.MethodPost, "/api/v1/resume", body)
	req.Header.Set("Content-Type", ct)
	rec := httptest.NewRecorder()
	interviewRouter(h).ServeHTTP(rec, req) // no user id in context

	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestUploadResume_RejectsNonPDF(t *testing.T) {
	called := false
	svc := &fakeSvc{
		uploadResumeFunc: func(context.Context, string, []byte, string) (domain.ResumeStatus, error) {
			called = true
			return domain.ResumeStatus{}, nil
		},
	}
	body, ct := resumeMultipart(t, "resume", []byte("this is not a pdf"))
	h := NewInterviewHandler(svc, discardLogger())
	req := withUserID(httptest.NewRequest(http.MethodPost, "/api/v1/resume", body), "user_1")
	req.Header.Set("Content-Type", ct)
	rec := httptest.NewRecorder()
	interviewRouter(h).ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.False(t, called, "service must not be reached when the file is not a PDF")
}

func TestUploadResume_HappyPath(t *testing.T) {
	uploadedAt := time.Now().UTC().Truncate(time.Second)
	svc := &fakeSvc{
		uploadResumeFunc: func(_ context.Context, userID string, pdf []byte, mimeType string) (domain.ResumeStatus, error) {
			require.Equal(t, "user_1", userID)
			require.Equal(t, "application/pdf", mimeType)
			require.True(t, bytes.HasPrefix(pdf, []byte("%PDF-")))
			return domain.ResumeStatus{Attached: true, UploadedAt: uploadedAt}, nil
		},
	}
	body, ct := resumeMultipart(t, "resume", []byte("%PDF-1.7 resume bytes"))
	h := NewInterviewHandler(svc, discardLogger())
	req := withUserID(httptest.NewRequest(http.MethodPost, "/api/v1/resume", body), "user_1")
	req.Header.Set("Content-Type", ct)
	rec := httptest.NewRecorder()
	interviewRouter(h).ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var got resumeResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	require.True(t, got.Attached)
	require.NotNil(t, got.UploadedAt)
}

func TestGetResume_ReportsNotAttached(t *testing.T) {
	svc := &fakeSvc{
		getResumeFunc: func(context.Context, string) (domain.ResumeStatus, error) {
			return domain.ResumeStatus{Attached: false}, nil
		},
	}
	h := NewInterviewHandler(svc, discardLogger())
	req := withUserID(httptest.NewRequest(http.MethodGet, "/api/v1/resume", nil), "user_1")
	rec := httptest.NewRecorder()
	interviewRouter(h).ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var got resumeResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	require.False(t, got.Attached)
	require.Nil(t, got.UploadedAt, "uploadedAt is null when no resume is on file")
}

func TestDeleteResume_HappyPath(t *testing.T) {
	called := false
	svc := &fakeSvc{
		deleteResumeFunc: func(_ context.Context, userID string) error {
			require.Equal(t, "user_1", userID)
			called = true
			return nil
		},
	}
	h := NewInterviewHandler(svc, discardLogger())
	req := withUserID(httptest.NewRequest(http.MethodDelete, "/api/v1/resume", nil), "user_1")
	rec := httptest.NewRecorder()
	interviewRouter(h).ServeHTTP(rec, req)

	require.Equal(t, http.StatusNoContent, rec.Code)
	require.True(t, called)
}
