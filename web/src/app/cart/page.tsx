import type { Metadata } from "next";
import Link from "next/link";
import { cookies } from "next/headers";
import { Suspense } from "react";
import { CART_COOKIE, MAX_QUANTITY, getCart, isCartId, type CartLine } from "@/lib/cart";
import { formatMoney } from "@/lib/format";
import { setLineQuantity } from "./actions";

export const metadata: Metadata = { title: "Your cart" };

// Same pattern as the other pages: a synchronous shell, with the part that
// reads the cookie and calls the cart service inside <Suspense>.
export default function CartPage() {
  return (
    <div>
      <h1 className="text-3xl font-semibold tracking-tight">Your cart</h1>
      <div className="mt-6">
        <Suspense fallback={<div className="h-40 animate-pulse rounded-lg bg-neutral-100" />}>
          <CartView />
        </Suspense>
      </div>
    </div>
  );
}

function Empty() {
  return (
    <p className="text-neutral-500">
      Your cart is empty.{" "}
      <Link href="/" className="underline">
        Keep shopping
      </Link>
    </p>
  );
}

async function CartView() {
  const id = (await cookies()).get(CART_COOKIE)?.value;
  if (!isCartId(id)) return <Empty />;

  const result = await getCart(id);
  if (!result.ok) {
    return <p className="text-red-700">We could not load your cart: {result.error}</p>;
  }
  const cart = result.cart;
  if (cart.lines.length === 0) return <Empty />;

  return (
    <div className="grid gap-8 md:grid-cols-[1fr_20rem]">
      <ul className="divide-y divide-neutral-200 border-y border-neutral-200">
        {cart.lines.map((line) => (
          <Line key={line.sku} line={line} currency={cart.currency} />
        ))}
      </ul>

      <aside className="h-fit rounded-lg border border-neutral-200 p-5">
        <h2 className="font-semibold">Order summary</h2>
        <dl className="mt-3 flex justify-between text-sm">
          <dt>
            Subtotal ({cart.itemCount} item{cart.itemCount === 1 ? "" : "s"})
          </dt>
          <dd className="font-semibold">{formatMoney(cart.subtotalCents, cart.currency)}</dd>
        </dl>
        <p className="mt-2 text-xs text-neutral-500">
          Prices come from the catalog each time this page loads. Shipping and tax come at checkout.
        </p>
        <button
          type="button"
          disabled
          className="mt-4 w-full rounded bg-neutral-900 px-4 py-3 text-white opacity-40"
        >
          Checkout (coming in Phase 3)
        </button>
      </aside>
    </div>
  );
}

// A form whose action is a Server Action. Submitting it posts the hidden fields
// to the server, which updates the cart and re-renders this page. No client
// JavaScript is needed, so it works even before the page has hydrated.
function QuantityForm({
  sku,
  quantity,
  label,
  disabled,
  children,
}: {
  sku: string;
  quantity: number;
  label: string;
  disabled?: boolean;
  children: React.ReactNode;
}) {
  return (
    <form action={setLineQuantity}>
      <input type="hidden" name="sku" value={sku} />
      <input type="hidden" name="quantity" value={quantity} />
      <button
        type="submit"
        aria-label={label}
        disabled={disabled}
        className="rounded border border-neutral-300 px-2.5 py-1 text-sm hover:border-neutral-900 disabled:opacity-30"
      >
        {children}
      </button>
    </form>
  );
}

function Line({ line, currency }: { line: CartLine; currency: string }) {
  return (
    <li className="flex gap-4 py-5">
      <div className="h-28 w-24 shrink-0 overflow-hidden rounded bg-neutral-100">
        {line.image && (
          // eslint-disable-next-line @next/next/no-img-element
          <img src={line.image.url} alt={line.image.alt} className="h-full w-full object-cover" />
        )}
      </div>

      <div className="flex-1">
        {line.available && line.productHandle ? (
          <Link href={`/products/${line.productHandle}`} className="font-medium hover:underline">
            {line.title}
          </Link>
        ) : (
          <p className="font-medium text-neutral-500">{line.title}</p>
        )}
        {line.variantTitle && <p className="text-sm text-neutral-500">{line.variantTitle}</p>}
        <p className="text-xs text-neutral-400">SKU {line.sku}</p>

        {line.available ? (
          <div className="mt-3 flex items-center gap-2">
            <QuantityForm sku={line.sku} quantity={line.quantity - 1} label="Decrease quantity">
              −
            </QuantityForm>
            <span className="w-8 text-center text-sm" aria-live="polite">
              {line.quantity}
            </span>
            <QuantityForm
              sku={line.sku}
              quantity={line.quantity + 1}
              label="Increase quantity"
              disabled={line.quantity >= MAX_QUANTITY}
            >
              +
            </QuantityForm>
            <QuantityForm sku={line.sku} quantity={0} label="Remove from cart">
              Remove
            </QuantityForm>
          </div>
        ) : (
          <div className="mt-3 flex items-center gap-3 text-sm">
            <span className="text-red-700">This item is no longer sold.</span>
            <QuantityForm sku={line.sku} quantity={0} label="Remove from cart">
              Remove
            </QuantityForm>
          </div>
        )}
      </div>

      {line.available && (
        <div className="text-right text-sm">
          <p className="font-semibold">{formatMoney(line.lineTotalCents, currency)}</p>
          {line.quantity > 1 && (
            <p className="text-neutral-500">{formatMoney(line.unitPriceCents, currency)} each</p>
          )}
        </div>
      )}
    </li>
  );
}
