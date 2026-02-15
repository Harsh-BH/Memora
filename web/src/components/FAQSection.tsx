"use client";

import { useState } from "react";
import { motion, AnimatePresence } from "framer-motion";

const faqs = [
    {
        question: "Is this just another RAG system?",
        answer:
            "No. Memora goes beyond simple retrieval-augmented generation. It implements a biologically-inspired Continuum Memory Architecture with episodic encoding (hippocampus), semantic consolidation (neocortex sleep cycles), and cognitive workspace assembly — giving your AI true persistent memory, not just search.",
    },
    {
        question: "Can it integrate with my current LLM stack?",
        answer:
            "Absolutely. Memora exposes simple REST APIs for ingestion and querying. You can plug it into any LLM pipeline — OpenAI, Anthropic, open-source models — by sending conversation turns to the ingest endpoint and querying the workspace before generating responses.",
    },
    {
        question: "How accurate is the memory recall?",
        answer:
            "Memora uses DIG (Diagnostic Information Gain) reranking combined with Lagrangian knapsack optimization to maximize recall precision within your token budget. In benchmarks, it achieves 96% accuracy on contextual recall tasks, significantly outperforming naive vector search.",
    },
    {
        question: "Is it safe to store my users' conversation data?",
        answer:
            "Yes. All data is fully partitioned by user_id across both Qdrant and Neo4j stores. Multi-tenant support is built-in via X-Tenant-ID headers. Your users' memories are completely isolated and never cross-contaminate.",
    },
    {
        question: "Can I try it for free?",
        answer:
            "Yes! The entire architecture is open-source. You can self-host with Docker Compose in under 5 minutes. Just run `docker compose up -d`, set your OpenAI key, and start ingesting memories immediately.",
    },
];

function FAQItem({
    question,
    answer,
    isOpen,
    onToggle,
}: {
    question: string;
    answer: string;
    isOpen: boolean;
    onToggle: () => void;
}) {
    return (
        <div className="border-b border-terracotta/10">
            <button
                onClick={onToggle}
                className="w-full flex items-center justify-between py-4 px-2 text-left group cursor-pointer"
            >
                <span className="text-sm font-medium text-charcoal group-hover:text-terracotta transition-colors">
                    {question}
                </span>
                <motion.span
                    animate={{ rotate: isOpen ? 45 : 0 }}
                    transition={{ duration: 0.2 }}
                    className="text-terracotta text-lg flex-shrink-0 ml-4"
                >
                    +
                </motion.span>
            </button>
            <AnimatePresence>
                {isOpen && (
                    <motion.div
                        initial={{ height: 0, opacity: 0 }}
                        animate={{ height: "auto", opacity: 1 }}
                        exit={{ height: 0, opacity: 0 }}
                        transition={{ duration: 0.3, ease: "easeOut" }}
                        className="overflow-hidden"
                    >
                        <p className="text-sm text-charcoal/70 leading-relaxed px-2 pb-4">
                            {answer}
                        </p>
                    </motion.div>
                )}
            </AnimatePresence>
        </div>
    );
}

export default function FAQSection() {
    const [openIndex, setOpenIndex] = useState<number | null>(null);

    return (
        <section className="py-24 px-6 relative bg-cream">
            <div className="max-w-3xl mx-auto">
                {/* Heading */}
                <motion.div
                    initial={{ opacity: 0, y: 20 }}
                    whileInView={{ opacity: 1, y: 0 }}
                    viewport={{ once: true }}
                    transition={{ duration: 0.6 }}
                    className="text-center mb-12"
                >
                    <h2 className="text-4xl md:text-5xl font-bold text-charcoal leading-tight">
                        Commonly Asked Questions
                    </h2>
                    <p className="mt-3 text-charcoal/60 text-sm">
                        See what our developers are asking about Memora.
                    </p>
                </motion.div>

                {/* FAQ + Decorations wrapper */}
                <div className="relative">
                    {/* Floral decorations */}
                    <div className="absolute -left-24 top-0 hidden lg:block pointer-events-none">
                        <img
                            src="/flower-orange.png"
                            alt=""
                            className="w-20 h-auto opacity-70"
                        />
                    </div>
                    <div className="absolute -right-24 -top-8 hidden lg:block pointer-events-none">
                        <img
                            src="/flower-yellow.png"
                            alt=""
                            className="w-20 h-auto opacity-70"
                        />
                    </div>

                    {/* FAQ List */}
                    <motion.div
                        initial={{ opacity: 0, y: 20 }}
                        whileInView={{ opacity: 1, y: 0 }}
                        viewport={{ once: true }}
                        transition={{ duration: 0.6, delay: 0.2 }}
                        className="bg-white rounded-2xl border border-terracotta/20 shadow-sm px-6"
                    >
                        {faqs.map((faq, i) => (
                            <FAQItem
                                key={i}
                                question={faq.question}
                                answer={faq.answer}
                                isOpen={openIndex === i}
                                onToggle={() =>
                                    setOpenIndex(openIndex === i ? null : i)
                                }
                            />
                        ))}
                    </motion.div>

                    {/* Bottom row: Contact + Mascot */}
                    <motion.div
                        initial={{ opacity: 0, y: 20 }}
                        whileInView={{ opacity: 1, y: 0 }}
                        viewport={{ once: true }}
                        transition={{ duration: 0.6, delay: 0.4 }}
                        className="flex items-end justify-between mt-8"
                    >
                        <div>
                            <p className="text-sm text-charcoal/60 mb-3">
                                If you still can&apos;t find your answer, please contact us.
                            </p>
                            <button className="bg-terracotta text-white text-sm px-5 py-2.5 rounded-lg font-medium hover:bg-terracotta/90 transition-colors cursor-pointer">
                                Contact Us
                            </button>
                        </div>
                        <img
                            src="/mascot-headphone.png"
                            alt="Support mascot"
                            className="w-28 h-28 object-contain"
                        />
                    </motion.div>
                </div>
            </div>
        </section>
    );
}
