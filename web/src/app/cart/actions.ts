"use server";

// Server Actions: functions that run on the server but can be called from a
// button or form in the browser. Next.js turns each call into a POST request.
// They are public endpoints, so treat every argument as untrusted input; the
// cart service validates everything again.

import { revalidatePath } from "next/cache";
import { cookies } from "next/headers";
import {
  CART_COOKIE,
  CART_COOKIE_MAX_AGE,
  addItem,
  createCart,
  isCartId,
  setQuantity,
} from "@/lib/cart";

export type AddResult =
  | { ok: true; itemCount: number }
  | { ok: false; error: string };

// addToCart puts one variant in the shopper's cart, creating the cart (and its
// cookie) on first use. Cookies can only be written from an action or a route
// handler, never while a page renders, which is why this lives here.
export async function addToCart(
  sku: string,
  quantity: number,
  idempotencyKey: string,
): Promise<AddResult> {
  if (typeof sku !== "string" || typeof idempotencyKey !== "string" || !Number.isInteger(quantity)) {
    return { ok: false, error: "Invalid request." };
  }

  const jar = await cookies();
  let id = jar.get(CART_COOKIE)?.value;
  if (!isCartId(id)) {
    try {
      id = await createCart();
    } catch {
      return { ok: false, error: "The cart is unavailable right now. Please try again." };
    }
    jar.set(CART_COOKIE, id, {
      httpOnly: true,
      sameSite: "lax",
      secure: process.env.NODE_ENV === "production",
      path: "/",
      maxAge: CART_COOKIE_MAX_AGE,
    });
  }

  let result;
  try {
    result = await addItem(id, sku, quantity, idempotencyKey);
  } catch {
    return { ok: false, error: "The cart is unavailable right now. Please try again." };
  }
  if (!result.ok) return { ok: false, error: result.error };

  // Re-render the layout so the header's cart count updates.
  revalidatePath("/", "layout");
  return { ok: true, itemCount: result.cart.itemCount };
}

// setLineQuantity is the form action behind the +, - and Remove buttons on the
// cart page. Each button is a tiny form with hidden `sku` and `quantity` fields.
export async function setLineQuantity(formData: FormData): Promise<void> {
  const sku = formData.get("sku");
  const quantity = Number(formData.get("quantity"));
  const id = (await cookies()).get(CART_COOKIE)?.value;
  if (typeof sku !== "string" || !Number.isInteger(quantity) || !isCartId(id)) return;

  const result = await setQuantity(id, sku, quantity);
  if (!result.ok) throw new Error(result.error); // shown by app/cart/error.tsx
  revalidatePath("/", "layout");
}
