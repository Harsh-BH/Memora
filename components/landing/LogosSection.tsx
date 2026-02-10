const logos = ["Arcstone", "Northpath", "Lumio", "Kitehouse", "Vantage", "Layerly"];

export function LogosSection() {
  return (
    <section className="px-4 py-14 sm:px-6 lg:px-8">
      <div className="mx-auto max-w-6xl">
        <p className="text-center text-xs font-semibold uppercase tracking-[0.18em] text-gray-500">
          Trusted by teams at
        </p>
        <div className="mt-8 grid grid-cols-2 gap-4 sm:grid-cols-3 lg:grid-cols-6">
          {logos.map((logo) => (
            <div
              key={logo}
              className="flex items-center justify-center rounded-2xl border border-gray-200 bg-white px-4 py-5 text-sm font-semibold text-gray-600 shadow-sm"
            >
              {logo}
            </div>
          ))}
        </div>
      </div>
    </section>
  );
}
