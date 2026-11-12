"use client";

// An error boundary for everything under /cart. Next.js renders it when a
// component or Server Action in this route throws, and `reset` retries.
export default function CartError({
  error,
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  return (
    <div className="py-12 text-center">
      <h1 className="text-2xl font-semibold">We couldn&apos;t update your cart</h1>
      <p className="mt-2 text-neutral-500">{error.message}</p>
      <button
        type="button"
        onClick={reset}
        className="mt-6 rounded bg-neutral-900 px-4 py-2 text-white"
      >
        Try again
      </button>
    </div>
  );
}
