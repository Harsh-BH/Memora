import { SectionHeader } from "@/components/ui/SectionHeader";

const features = [
  {
    title: "Unified knowledge",
    description: "Keep docs, notes, and decisions linked so context is never lost.",
  },
  {
    title: "Smart reminders",
    description: "Turn ideas into assignable next steps with lightweight follow-up prompts.",
  },
  {
    title: "Fast collaboration",
    description: "Comment, mention teammates, and ship updates from one shared workspace.",
  },
  {
    title: "Effortless search",
    description: "Find past conversations and project memory in seconds.",
  },
];

export function FeaturesSection() {
  return (
    <section className="px-4 py-16 sm:px-6 lg:px-8 lg:py-24">
      <div className="mx-auto max-w-6xl">
        <SectionHeader
          eyebrow="Capabilities"
          title="Powerful features with a calm interface"
          description="Memora focuses your team on outcomes while quietly handling the details behind the scenes."
        />
        <div className="mt-10 grid gap-5 sm:grid-cols-2">
          {features.map((feature) => (
            <article key={feature.title} className="rounded-3xl border border-gray-200 bg-white p-6 shadow-sm">
              <h3 className="text-lg font-semibold text-gray-900">{feature.title}</h3>
              <p className="mt-3 text-sm text-gray-600">{feature.description}</p>
            </article>
          ))}
        </div>
      </div>
    </section>
  );
}
