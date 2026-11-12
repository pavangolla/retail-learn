"use client";

import Link from "next/link";
import { useState, useTransition } from "react";
import { addToCart } from "@/app/cart/actions";
import type { Option, Variant } from "@/lib/catalog";
import { formatMoney } from "@/lib/format";

// A variant is one purchasable combination of options (Color + Size). The shopper
// chooses a value per option, and we look up the variant that matches.
export function VariantPicker({
  options,
  variants,
}: {
  options: Option[];
  variants: Variant[];
}) {
  // Start with the first variant's choices, e.g. { Color: "Red", Size: "S" }.
  const [selected, setSelected] = useState<Record<string, string>>(
    variants[0]?.selectedOptions ?? {},
  );

  const variant = variants.find((v) =>
    options.every((o) => v.selectedOptions[o.name] === selected[o.name]),
  );

  const onSale =
    variant?.compareAtCents !== undefined &&
    variant.compareAtCents > variant.priceCents;

  // useTransition marks the server call as "pending" without freezing the page,
  // which lets us disable the button while the request is in flight.
  const [pending, startTransition] = useTransition();
  const [message, setMessage] = useState<{ ok: boolean; text: string } | null>(null);

  function onAdd() {
    if (!variant) return;
    // One key per click: if this request is retried on the way, the cart service
    // sees the same key and does not add the item twice.
    const key = crypto.randomUUID();
    setMessage(null);
    startTransition(async () => {
      const res = await addToCart(variant.sku, 1, key);
      setMessage(
        res.ok
          ? { ok: true, text: `Added. Your cart has ${res.itemCount} item${res.itemCount === 1 ? "" : "s"}.` }
          : { ok: false, text: res.error },
      );
    });
  }

  return (
    <div className="mt-4">
      {variant ? (
        <p className="text-2xl">
          <span className="font-semibold">
            {formatMoney(variant.priceCents, variant.currency)}
          </span>
          {onSale && (
            <span className="ml-3 text-lg text-neutral-400 line-through">
              {formatMoney(variant.compareAtCents!, variant.currency)}
            </span>
          )}
        </p>
      ) : (
        <p className="text-lg text-neutral-500">This combination is unavailable.</p>
      )}

      {options.map((o) => (
        <fieldset key={o.name} className="mt-5">
          <legend className="mb-2 text-sm font-medium">
            {o.name}: <span className="font-normal">{selected[o.name]}</span>
          </legend>
          <div className="flex flex-wrap gap-2">
            {o.values.map((val) => (
              <button
                key={val}
                type="button"
                aria-pressed={selected[o.name] === val}
                onClick={() => setSelected((s) => ({ ...s, [o.name]: val }))}
                className={`rounded border px-3 py-1.5 text-sm ${
                  selected[o.name] === val
                    ? "border-neutral-900 bg-neutral-900 text-white"
                    : "border-neutral-300 hover:border-neutral-900"
                }`}
              >
                {val}
              </button>
            ))}
          </div>
        </fieldset>
      ))}

      {variant && <p className="mt-4 text-xs text-neutral-500">SKU {variant.sku}</p>}

      <button
        type="button"
        onClick={onAdd}
        disabled={!variant || pending}
        className="mt-6 w-full rounded bg-neutral-900 px-4 py-3 text-white disabled:opacity-40"
      >
        {pending ? "Adding..." : "Add to cart"}
      </button>

      <p
        role="status"
        className={`mt-3 text-sm ${message && !message.ok ? "text-red-700" : "text-green-700"}`}
      >
        {message?.text}
        {message?.ok && (
          <>
            {" "}
            <Link href="/cart" className="underline">
              View cart
            </Link>
          </>
        )}
      </p>
    </div>
  );
}
