// Client for the Go cart service, plus the cookie settings that remember which
// cart is the shopper's. Server-side only: the browser never talks to the cart
// service, it only holds an httpOnly cookie that the Next.js server reads.

const BASE = process.env.CART_URL ?? "http://localhost:8082";

// The cookie holds the cart id. HttpOnly keeps page JavaScript from reading it;
// the max age mirrors the cart service's 30-day expiry.
export const CART_COOKIE = "cart_id";
export const CART_COOKIE_MAX_AGE = 60 * 60 * 24 * 30;

// Matches the cart service's own limit. The service is what enforces it; the UI
// only uses this to disable a "+" button that would be refused.
export const MAX_QUANTITY = 10;

// A cart id is 32 hex characters. Checking before use keeps a tampered cookie
// from being sent anywhere.
export function isCartId(v: string | undefined): v is string {
  return typeof v === "string" && /^[0-9a-f]{32}$/.test(v);
}

// ---- Types: they mirror the JSON the cart service returns ----

export type CartLine = {
  sku: string;
  productHandle?: string;
  title: string;
  variantTitle?: string;
  image?: { url: string; alt: string };
  quantity: number;
  unitPriceCents: number;
  compareAtCents?: number;
  lineTotalCents: number;
  available: boolean;
};

export type Cart = {
  id: string;
  lines: CartLine[];
  itemCount: number;
  subtotalCents: number;
  currency: string;
};

export type CartResult =
  | { ok: true; cart: Cart }
  | { ok: false; status: number; error: string };

// ---- Calls ----

async function send(
  path: string,
  init: RequestInit & { retry?: boolean } = {},
): Promise<Response> {
  const { retry, ...rest } = init;
  // A request is retried once on a network failure or a 5xx, but only when the
  // caller says it is safe (retry: true) -- that is, the call is idempotent.
  const attempts = retry ? 2 : 1;
  let lastError: unknown;
  for (let i = 0; i < attempts; i++) {
    try {
      const res = await fetch(`${BASE}${path}`, {
        ...rest,
        cache: "no-store",
        signal: AbortSignal.timeout(5000),
      });
      if (res.status < 500 || i === attempts - 1) return res;
    } catch (err) {
      lastError = err;
    }
  }
  throw lastError ?? new Error("cart service unavailable");
}

async function toResult(res: Response): Promise<CartResult> {
  if (res.ok) return { ok: true, cart: (await res.json()) as Cart };
  let error = "Something went wrong. Please try again.";
  try {
    const body = (await res.json()) as { error?: string };
    if (body.error && res.status < 500) error = body.error;
  } catch {
    // keep the generic message
  }
  return { ok: false, status: res.status, error };
}

// createCart asks the cart service for a fresh id. Nothing is stored until the
// first item is added.
export async function createCart(): Promise<string> {
  const res = await send("/carts", { method: "POST" });
  if (!res.ok) throw new Error(`cart service answered ${res.status}`);
  return ((await res.json()) as { id: string }).id;
}

export async function getCart(id: string): Promise<CartResult> {
  return toResult(await send(`/carts/${id}`));
}

// addItem adds `quantity` more of a SKU. The Idempotency-Key makes the retry in
// send() safe: if the first attempt reached the service but the answer was lost,
// the second attempt is recognized and does not add the item twice.
export async function addItem(
  id: string,
  sku: string,
  quantity: number,
  idempotencyKey: string,
): Promise<CartResult> {
  return toResult(
    await send(`/carts/${id}/items`, {
      method: "POST",
      headers: { "Content-Type": "application/json", "Idempotency-Key": idempotencyKey },
      body: JSON.stringify({ sku, quantity }),
      retry: true,
    }),
  );
}

// setQuantity makes a line exactly `quantity` (0 removes it). PUT is naturally
// idempotent, so it needs no key and can always be retried.
export async function setQuantity(
  id: string,
  sku: string,
  quantity: number,
): Promise<CartResult> {
  return toResult(
    await send(`/carts/${id}/items/${encodeURIComponent(sku)}`, {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ quantity }),
      retry: true,
    }),
  );
}
