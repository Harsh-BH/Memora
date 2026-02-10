import { CTAButton } from "@/components/ui/CTAButton";

export function MidCTASection() {
  return (
    <section className="px-4 py-16 sm:px-6 lg:px-8 lg:py-24">
      <div className="mx-auto max-w-5xl rounded-3xl border border-gray-200 bg-white p-8 shadow-lg sm:p-12">
        <div className="grid items-center gap-8 lg:grid-cols-[2fr,1fr]">
          <div>
            <p className="text-xs font-semibold uppercase tracking-[0.2em] text-gray-500">Get momentum</p>
            <h2 className="mt-4 text-3xl font-bold tracking-tight text-gray-900 sm:text-4xl">
              Ready to run your team on shared memory?
            </h2>
            <p className="mt-4 text-base text-gray-600">
              Start with a free workspace and invite your team in minutes.
            </p>
          </div>
          <div className="flex flex-col gap-3 sm:flex-row lg:flex-col">
            <CTAButton href="#">Try Memora</CTAButton>
            <CTAButton href="#" variant="secondary">
              Contact sales
            </CTAButton>
          </div>
        </div>
      </div>
    </section>
  );
}
