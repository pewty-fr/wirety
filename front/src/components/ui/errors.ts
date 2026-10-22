import axios from 'axios';

/** The message to show for a failed API call: the server's error, else a generic one. */
export function apiErrorMessage(error: unknown, fallback = 'Something went wrong. Please try again.'): string {
  if (axios.isAxiosError(error)) {
    const data = error.response?.data as { error?: string; message?: string } | undefined;
    if (data?.error) return data.error;
    if (data?.message) return data.message;
    if (error.response?.status === 403) return 'You are not allowed to do this.';
  }
  if (error instanceof Error && error.message) return error.message;
  return fallback;
}
