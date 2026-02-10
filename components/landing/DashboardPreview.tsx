import { CardWrapper } from "@/components/ui/CardWrapper";
import { SectionHeader } from "@/components/ui/SectionHeader";

export function DashboardPreview() {
  return (
    <section className="px-4 py-16 sm:px-6 lg:px-8 lg:py-24">
      <div className="mx-auto max-w-6xl">
        <SectionHeader
          centered
          eyebrow="Workspace visibility"
          title="Everything your team needs in one calm dashboard"
          description="Track priorities, monitor progress, and access critical context without digging through tabs."
        />
        <CardWrapper className="mt-12 shadow-lg">
          <div className="grid gap-6 lg:grid-cols-[2fr,1fr]">
            <div className="rounded-2xl border border-gray-200 bg-gray-50 p-6">
              <p className="text-sm font-semibold text-gray-900">Weekly Momentum</p>
              <div className="mt-6 grid gap-4 sm:grid-cols-3">
                {[
                  ["Completed", "34"],
                  ["In progress", "12"],
                  ["Blocked", "3"],
                ].map(([label, value]) => (
                  <div key={label} className="rounded-2xl border border-gray-200 bg-white p-4 shadow-sm">
                    <p className="text-xs uppercase tracking-wide text-gray-500">{label}</p>
                    <p className="mt-2 text-2xl font-bold text-gray-900">{value}</p>
                  </div>
                ))}
              </div>
            </div>
            <div className="rounded-2xl border border-gray-200 bg-white p-6 shadow-sm">
              <p className="text-sm font-semibold text-gray-900">Top priorities</p>
              <ul className="mt-4 space-y-3 text-sm text-gray-600">
                <li className="rounded-xl border border-gray-200 px-3 py-2">Finalize onboarding flow</li>
                <li className="rounded-xl border border-gray-200 px-3 py-2">Publish launch checklist</li>
                <li className="rounded-xl border border-gray-200 px-3 py-2">Align Q3 roadmap themes</li>
              </ul>
            </div>
          </div>
        </CardWrapper>
      </div>
    </section>
  );
}
