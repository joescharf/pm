const DEV_API_BASE = "http://localhost:8080";

function getBaseURL(): string {
  if (
    import.meta.env?.MODE === "development" ||
    window.location.port === "3000" ||
    window.location.port === "3001"
  ) {
    return DEV_API_BASE;
  }
  return "";
}

const BASE = getBaseURL();

export async function apiFetch<T>(
  path: string,
  init?: RequestInit,
): Promise<T> {
  const url = `${BASE}${path}`;
  const res = await fetch(url, {
    ...init,
    headers: {
      "Content-Type": "application/json",
      ...init?.headers,
    },
  });
  if (!res.ok) {
    const body = await res.text();
    throw new Error(`${res.status}: ${body}`);
  }
  if (res.status === 204) {
    return undefined as T;
  }
  return res.json();
}

export { BASE };
