import { SectionHeader } from "@/components/ui/SectionHeader";

const faq = [
  {
    question: "Can I migrate from Notion or docs?",
    answer: "Yes. Import Markdown, plain text, and CSV exports to get started quickly.",
  },
  {
    question: "Is Memora suitable for small teams?",
    answer: "Absolutely. The free plan supports up to 10 collaborators with core features included.",
  },
  {
    question: "Do you offer enterprise controls?",
    answer: "Enterprise includes SSO, audit logs, and advanced role-based permissions.",
  },
];

export function FAQSection() {
  return (
    <section className="px-4 py-16 sm:px-6 lg:px-8 lg:py-24">
      <div className="mx-auto max-w-4xl">
        <SectionHeader
          centered
          eyebrow="FAQ"
          title="Common questions"
          description="Answers to the most frequent questions from teams evaluating Memora."
        />
        <div className="mt-10 space-y-4">
          {faq.map((item) => (
            <article key={item.question} className="rounded-2xl border border-gray-200 bg-white p-6 shadow-sm">
              <h3 className="text-base font-semibold text-gray-900">{item.question}</h3>
              <p className="mt-2 text-sm text-gray-600">{item.answer}</p>
            </article>
          ))}
        </div>
      </div>
    </section>
  );
}
