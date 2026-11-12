import Link from "next/link";
import { cookies } from "next/headers";
import { CART_COOKIE, getCart, isCartId } from "@/lib/cart";

// Reads the cart cookie, so it is dynamic and must sit inside <Suspense>.
// A broken cart service must not break every page, so errors just hide the count.
export async function CartLink() {
  let count: number | null = 0;
  const id = (await cookies()).get(CART_COOKIE)?.value;
  if (isCartId(id)) {
    try {
      const res = await getCart(id);
      count = res.ok ? res.cart.itemCount : null;
    } catch {
      count = null;
    }
  }
  return (
    <Link href="/cart" className="text-sm font-medium hover:underline">
      Cart{count ? ` (${count})` : ""}
    </Link>
  );
}

export function CartLinkFallback() {
  return <span className="text-sm font-medium">Cart</span>;
}
