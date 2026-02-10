import { CTAButton } from "@/components/ui/CTAButton";

export function HeroSection() {
  return (
    <section className="px-4 py-16 sm:px-6 lg:px-8 lg:py-24">
      <div className="mx-auto grid max-w-6xl items-center gap-12 lg:grid-cols-2">
        <div>
          <p className="mb-4 inline-flex rounded-2xl border border-gray-200 bg-white px-3 py-1 text-xs font-semibold uppercase tracking-[0.18em] text-gray-500 shadow-sm">
            The modern memory workspace
          </p>
          <h1 className="text-4xl font-bold tracking-tight text-gray-900 sm:text-5xl lg:text-6xl">
            Capture ideas instantly and turn them into action.
          </h1>
          <p className="mt-6 max-w-xl text-lg text-gray-600">
            Memora keeps your notes, tasks, and context connected so your team can move from thought
            to execution without losing momentum.
          </p>
          <div className="mt-8 flex flex-col gap-3 sm:flex-row">
            <CTAButton href="#">Start for free</CTAButton>
            <CTAButton href="#" variant="secondary">
              Book a demo
            </CTAButton>
          </div>
        </div>
        <div className="rounded-3xl border border-gray-200 bg-white p-4 shadow-lg sm:p-6">
          <div className="rounded-2xl border border-gray-200 bg-gray-50 p-5">
            <p className="text-sm font-semibold text-gray-800">Today</p>
            <ul className="mt-4 space-y-3">
              {[
                "Refine launch brief with product marketing",
                "Review customer interview notes",
                "Share sprint recap with leadership",
              ].map((item) => (
                <li key={item} className="rounded-2xl border border-gray-200 bg-white px-4 py-3 text-sm text-gray-700 shadow-sm">
                  {item}
                </li>
              ))}
            </ul>
          </div>
        </div>
      </div>
    </section>
  );
}
