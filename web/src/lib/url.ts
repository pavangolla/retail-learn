// Filters, sorting and paging live in the URL query string, so every view is a
// plain link: shareable, back-button friendly, and no client JavaScript.

export type SearchParams = Record<string, string | string[] | undefined>;

export function first(v: string | string[] | undefined): string | undefined {
  return Array.isArray(v) ? v[0] : v;
}

// href builds `path?...` from the current params with some keys changed.
// A change of `undefined` removes the key. Changing any filter or sort drops
// `after`, because a cursor is only valid for the view that issued it.
export function href(
  path: string,
  current: SearchParams,
  changes: Record<string, string | undefined>,
): string {
  const u = new URLSearchParams();
  for (const [k, v] of Object.entries(current)) {
    if (k in changes || v === undefined) continue;
    for (const item of Array.isArray(v) ? v : [v]) u.append(k, item);
  }
  for (const [k, v] of Object.entries(changes)) {
    if (v !== undefined) u.set(k, v);
  }
  if (!("after" in changes)) u.delete("after");
  const s = u.toString();
  return s ? `${path}?${s}` : path;
}

export function toInt(v: string | undefined): number | undefined {
  if (!v) return undefined;
  const n = Number.parseInt(v, 10);
  return Number.isFinite(n) && n > 0 ? n : undefined;
}
