// Money is stored as integer cents everywhere. Convert only at the edge, when
// showing a price to a person.
export function formatMoney(cents: number, currency = "USD"): string {
  return new Intl.NumberFormat("en-US", { style: "currency", currency }).format(
    cents / 100,
  );
}

// "silk" -> "Silk", "best-seller" -> "Best seller"
export function humanize(tag: string): string {
  const s = tag.replace(/-/g, " ");
  return s.charAt(0).toUpperCase() + s.slice(1);
}
