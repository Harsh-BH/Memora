import { SectionHeader } from "@/components/ui/SectionHeader";

const steps = [
  {
    title: "Capture",
    description: "Drop in notes, links, or voice memos from anywhere.",
  },
  {
    title: "Organize",
    description: "Automatically group work by project, owner, and urgency.",
  },
  {
    title: "Execute",
    description: "Convert priorities into clear tasks and follow-through.",
  },
];

export function ThreeStepsSection() {
  return (
    <section className="px-4 py-16 sm:px-6 lg:px-8 lg:py-24">
      <div className="mx-auto max-w-6xl">
        <SectionHeader
          centered
          eyebrow="Workflow"
          title="Three simple steps from thought to shipped"
          description="A focused process that scales from solo builders to cross-functional teams."
        />
        <div className="mt-12 grid gap-5 md:grid-cols-3">
          {steps.map((step, index) => (
            <article key={step.title} className="rounded-3xl border border-gray-200 bg-white p-6 shadow-sm">
              <p className="text-sm font-semibold text-gray-500">0{index + 1}</p>
              <h3 className="mt-3 text-xl font-semibold text-gray-900">{step.title}</h3>
              <p className="mt-3 text-sm text-gray-600">{step.description}</p>
            </article>
          ))}
        </div>
      </div>
    </section>
  );
}
