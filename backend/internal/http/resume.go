package http

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/bhavishya3102/ai-mock-interview/backend/internal/auth"
	"github.com/bhavishya3102/ai-mock-interview/backend/internal/domain"
)

// maxResumeBytes caps the resume upload at 5 MiB — generous for a 1-2 page
// PDF, small enough to reject obvious abuse before the LLM round-trip.
const maxResumeBytes int64 = 5 << 20

// pdfMagic is the leading byte signature every PDF file carries. We validate
// against this rather than trusting the multipart Content-Type, which some
// browsers report as application/octet-stream.
var pdfMagic = []byte("%PDF-")

type resumeResponse struct {
	Attached   bool       `json:"attached"`
	UploadedAt *time.Time `json:"uploadedAt"`
}

func toResumeResponse(s domain.ResumeStatus) resumeResponse {
	r := resumeResponse{Attached: s.Attached}
	if !s.UploadedAt.IsZero() {
		t := s.UploadedAt
		r.UploadedAt = &t
	}
	return r
}

// UploadResume handles POST /api/v1/resume.
//
// Accepts a multipart upload with the PDF in the "resume" field. The file is
// sent to the LLM for one-time text extraction; the extracted text is stored
// on the user and reused when generating questions for future interviews.
func (h *InterviewHandler) UploadResume(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		h.respondErr(w, r, errors.New("missing user id"), domain.ErrUnauthorized)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxResumeBytes+(1<<20))
	if err := r.ParseMultipartForm(maxResumeBytes); err != nil {
		h.respondErr(w, r, errors.Join(domain.ErrValidation, err), nil)
		return
	}

	file, header, err := r.FormFile("resume")
	if err != nil {
		h.respondErr(w, r, errors.Join(domain.ErrValidation, err), nil)
		return
	}
	defer file.Close()

	if header.Size <= 0 {
		h.respondErr(w, r, errors.New("empty resume file"), domain.ErrValidation)
		return
	}
	if header.Size > maxResumeBytes {
		h.respondErr(w, r, errors.New("resume too large"), domain.ErrValidation)
		return
	}

	pdf, err := io.ReadAll(io.LimitReader(file, maxResumeBytes+1))
	if err != nil {
		h.respondErr(w, r, err, nil)
		return
	}
	if int64(len(pdf)) > maxResumeBytes {
		h.respondErr(w, r, errors.New("resume too large"), domain.ErrValidation)
		return
	}
	if !bytes.HasPrefix(pdf, pdfMagic) {
		h.respondErr(w, r, errors.New("file is not a PDF"), domain.ErrValidation)
		return
	}

	status, err := h.svc.UploadResume(r.Context(), userID, pdf, "application/pdf")
	if err != nil {
		h.respondErr(w, r, err, nil)
		return
	}
	writeJSON(w, h.log, http.StatusOK, toResumeResponse(status))
}

// GetResume handles GET /api/v1/resume — reports whether a resume is on file.
func (h *InterviewHandler) GetResume(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		h.respondErr(w, r, errors.New("missing user id"), domain.ErrUnauthorized)
		return
	}
	status, err := h.svc.GetResume(r.Context(), userID)
	if err != nil {
		h.respondErr(w, r, err, nil)
		return
	}
	writeJSON(w, h.log, http.StatusOK, toResumeResponse(status))
}

// DeleteResume handles DELETE /api/v1/resume — removes the stored resume.
func (h *InterviewHandler) DeleteResume(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		h.respondErr(w, r, errors.New("missing user id"), domain.ErrUnauthorized)
		return
	}
	if err := h.svc.DeleteResume(r.Context(), userID); err != nil {
		h.respondErr(w, r, err, nil)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
