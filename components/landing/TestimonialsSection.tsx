import { SectionHeader } from "@/components/ui/SectionHeader";

const testimonials = [
  {
    quote:
      "Memora became our operating layer in a week. We have fewer meetings and faster project handoffs.",
    name: "Priya N.",
    role: "Head of Product, Lumio",
  },
  {
    quote:
      "The interface is incredibly clean. Everyone from design to engineering actually wants to keep it updated.",
    name: "Jordan K.",
    role: "Engineering Manager, Arcstone",
  },
];

export function TestimonialsSection() {
  return (
    <section className="px-4 py-16 sm:px-6 lg:px-8 lg:py-24">
      <div className="mx-auto max-w-6xl">
        <SectionHeader
          centered
          eyebrow="Loved by teams"
          title="What customers say about Memora"
          description="Teams of every size rely on Memora to stay aligned without adding process overhead."
        />
        <div className="mt-10 grid gap-5 lg:grid-cols-2">
          {testimonials.map((item) => (
            <figure key={item.name} className="rounded-3xl border border-gray-200 bg-white p-7 shadow-sm">
              <blockquote className="text-base leading-7 text-gray-700">“{item.quote}”</blockquote>
              <figcaption className="mt-5 text-sm">
                <p className="font-semibold text-gray-900">{item.name}</p>
                <p className="text-gray-600">{item.role}</p>
              </figcaption>
            </figure>
          ))}
        </div>
      </div>
    </section>
  );
}
