import { apiFetch } from "./client";
import type { ResumeStatus } from "@/types/api";

export async function getResumeStatus(signal?: AbortSignal): Promise<ResumeStatus> {
  const init = signal ? { signal } : {};
  return apiFetch<ResumeStatus>("/resume", init);
}

export async function uploadResume(file: File): Promise<ResumeStatus> {
  const form = new FormData();
  form.append("resume", file, file.name || "resume.pdf");
  return apiFetch<ResumeStatus>("/resume", { method: "POST", body: form });
}

export async function deleteResume(): Promise<void> {
  await apiFetch<null>("/resume", { method: "DELETE" });
}
