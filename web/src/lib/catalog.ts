// A small client for the Go catalog service. These functions run on the server
// only (they are called from server components), so the browser never talks to
// the Go service directly: no CORS, and CATALOG_URL stays private.

const BASE = process.env.CATALOG_URL ?? "http://localhost:8081";

// ---- Types: they mirror the JSON the Go service returns ----

export type Image = { url: string; alt: string };

export type Summary = {
  handle: string;
  title: string;
  productType: string;
  tags: string[];
  image?: Image;
  priceFromCents: number;
  compareAtFromCents?: number;
  currency: string;
};

export type Option = { name: string; values: string[] };

export type Variant = {
  sku: string;
  title: string;
  selectedOptions: Record<string, string>;
  priceCents: number;
  compareAtCents?: number;
  currency: string;
};

export type Product = {
  handle: string;
  title: string;
  description: string;
  vendor: string;
  productType: string;
  tags: string[];
  options: Option[];
  variants: Variant[];
  images: Image[];
};

export type Collection = {
  handle: string;
  title: string;
  description: string;
  tag: string;
};

export type Page = { items: Summary[]; nextCursor?: string };

export type FacetCount = { value: string; count: number };

export type Facets = {
  total: number;
  productTypes: FacetCount[] | null;
  tags: FacetCount[] | null;
  priceMinCents: number;
  priceMaxCents: number;
};

// What a page can ask for. `tag` is repeatable; prices are in cents.
export type ListParams = {
  sort?: string;
  type?: string;
  tag?: string[];
  minPrice?: number;
  maxPrice?: number;
  q?: string;
  after?: string;
  limit?: number;
};

// ---- Fetching ----

// getJSON returns null on 404 so pages can call notFound(). Any other failure
// throws, which shows the nearest error boundary instead of a blank page.
async function getJSON<T>(path: string): Promise<T | null> {
  const res = await fetch(`${BASE}${path}`, {
    cache: "no-store", // always live data; there is no caching layer yet
    signal: AbortSignal.timeout(5000),
  });
  if (res.status === 404) return null;
  if (!res.ok) {
    throw new Error(`catalog ${path} responded ${res.status}`);
  }
  return (await res.json()) as T;
}

function query(p: ListParams): string {
  const u = new URLSearchParams();
  if (p.sort) u.set("sort", p.sort);
  if (p.type) u.set("type", p.type);
  p.tag?.forEach((t) => u.append("tag", t));
  if (p.minPrice) u.set("minPrice", String(p.minPrice));
  if (p.maxPrice) u.set("maxPrice", String(p.maxPrice));
  if (p.q) u.set("q", p.q);
  if (p.after) u.set("after", p.after);
  if (p.limit) u.set("limit", String(p.limit));
  const s = u.toString();
  return s ? `?${s}` : "";
}

export async function listCollections(): Promise<Collection[]> {
  const res = await getJSON<{ items: Collection[] }>("/collections");
  return res?.items ?? [];
}

export function getCollection(handle: string) {
  return getJSON<Collection>(`/collections/${encodeURIComponent(handle)}`);
}

export function listCollectionProducts(handle: string, p: ListParams) {
  return getJSON<Page>(
    `/collections/${encodeURIComponent(handle)}/products${query(p)}`,
  );
}

export function getCollectionFacets(handle: string, p: ListParams) {
  return getJSON<Facets>(
    `/collections/${encodeURIComponent(handle)}/facets${query(p)}`,
  );
}

export async function listProducts(p: ListParams): Promise<Page> {
  return (await getJSON<Page>(`/products${query(p)}`)) ?? { items: [] };
}

export function getProduct(handle: string) {
  return getJSON<Product>(`/products/${encodeURIComponent(handle)}`);
}
