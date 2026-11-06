"use client";

import { useState } from "react";
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

      {/* Cart arrives in Phase 2. */}
      <button
        type="button"
        disabled
        className="mt-6 w-full rounded bg-neutral-900 px-4 py-3 text-white opacity-40"
      >
        Add to cart (coming in Phase 2)
      </button>
    </div>
  );
}
