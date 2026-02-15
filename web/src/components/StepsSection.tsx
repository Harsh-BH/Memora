"use client";

import { useState } from "react";
import { motion, AnimatePresence } from "framer-motion";

/* ── Mini Mockup UIs ── */

function IngestMockup() {
    return (
        <div className="bg-white rounded-2xl p-5 shadow-lg border border-terracotta/20">
            <div className="flex items-center gap-2 mb-3">
                <div className="w-6 h-6 rounded-lg bg-terracotta/20 flex items-center justify-center">
                    <span className="text-[10px]">📄</span>
                </div>
                <span className="text-xs font-semibold text-terracotta">
                    Ingest Conversation
                </span>
                <span className="ml-auto text-[8px] px-2 py-0.5 rounded-full bg-beige/30 text-charcoal font-medium">
                    Live
                </span>
            </div>

            {/* Code-like input */}
            <div className="bg-cream rounded-xl p-4 font-mono text-[10px] leading-relaxed mb-3 border border-terracotta/20">
                <p className="text-charcoal/70">
                    <span className="text-terracotta">POST</span>{" "}
                    /api/v1/conversations/ingest
                </p>
                <p className="text-terracotta mt-1">{"{"}</p>
                <p className="text-charcoal/70 ml-3">
                    <span className="text-terracotta">&quot;user_id&quot;</span>:{" "}
                    <span className="text-charcoal">&quot;usr_abc123&quot;</span>,
                </p>
                <p className="text-charcoal/70 ml-3">
                    <span className="text-terracotta">&quot;messages&quot;</span>:{" "}
                    <span className="text-charcoal">[...]</span>
                </p>
                <p className="text-terracotta">{"}"}</p>
            </div>

            {/* Status indicators */}
            <div className="flex items-center gap-3">
                <div className="flex items-center gap-1.5">
                    <div className="w-1.5 h-1.5 rounded-full bg-terracotta animate-pulse" />
                    <span className="text-[9px] text-charcoal/60">Segmenting</span>
                </div>
                <div className="flex items-center gap-1.5">
                    <div className="w-1.5 h-1.5 rounded-full bg-beige" />
                    <span className="text-[9px] text-charcoal/60">3 episodes found</span>
                </div>
                <div className="flex items-center gap-1.5">
                    <div className="w-1.5 h-1.5 rounded-full bg-charcoal" />
                    <span className="text-[9px] text-charcoal/60">8 triples extracted</span>
                </div>
            </div>
        </div>
    );
}

function ConsolidateMockup() {
    return (
        <div className="bg-white rounded-2xl p-5 shadow-lg border border-terracotta/20">
            <div className="flex items-center gap-2 mb-3">
                <div className="w-6 h-6 rounded-lg bg-charcoal/10 flex items-center justify-center">
                    <span className="text-[10px]">🌙</span>
                </div>
                <span className="text-xs font-semibold text-terracotta">
                    Sleep Consolidation
                </span>
                <span className="ml-auto text-[8px] px-2 py-0.5 rounded-full bg-terracotta/10 text-terracotta font-medium">
                    Cycle #14
                </span>
            </div>

            {/* Two-panel view */}
            <div className="grid grid-cols-2 gap-3 mb-3">
                <div className="bg-cream rounded-xl p-3 border border-terracotta/20">
                    <p className="text-[9px] font-semibold text-terracotta mb-1.5">
                        ✓ Synthesized
                    </p>
                    <div className="space-y-1.5">
                        <div className="bg-white rounded-lg px-2 py-1 text-[8px] text-charcoal/70 border border-terracotta/10">
                            (User, works_at, <span className="text-terracotta">Google</span>)
                        </div>
                        <div className="bg-white rounded-lg px-2 py-1 text-[8px] text-charcoal/70 border border-terracotta/10">
                            (User, skilled_in, <span className="text-terracotta">Go</span>)
                        </div>
                    </div>
                </div>
                <div className="bg-cream rounded-xl p-3 border border-terracotta/20">
                    <p className="text-[9px] font-semibold text-terracotta mb-1.5">
                        ⚠ Conflicts
                    </p>
                    <div className="space-y-1.5">
                        <div className="bg-white rounded-lg px-2 py-1 text-[8px] text-charcoal/70 border border-terracotta/10">
                            <span className="line-through text-charcoal/40">lives_in: NYC</span>
                            <br />
                            <span className="text-terracotta">lives_in: SF</span>{" "}
                            <span className="text-[7px] text-charcoal/50">newer</span>
                        </div>
                    </div>
                </div>
            </div>

            {/* Progress */}
            <div>
                <div className="flex justify-between mb-1">
                    <span className="text-[9px] text-charcoal/60">Consolidation progress</span>
                    <span className="text-[9px] text-terracotta font-medium">78%</span>
                </div>
                <div className="bg-cream rounded-full h-1.5 overflow-hidden">
                    <motion.div
                        className="h-full bg-gradient-to-r from-terracotta to-beige rounded-full"
                        initial={{ width: "0%" }}
                        whileInView={{ width: "78%" }}
                        viewport={{ once: true }}
                        transition={{ duration: 1.5, ease: "easeOut" }}
                    />
                </div>
            </div>
        </div>
    );
}

function WorkspaceMockup() {
    return (
        <div className="bg-white rounded-2xl p-5 shadow-lg border border-terracotta/20">
            <div className="flex items-center gap-2 mb-3">
                <div className="w-6 h-6 rounded-lg bg-beige/30 flex items-center justify-center">
                    <span className="text-[10px]">⚡</span>
                </div>
                <span className="text-xs font-semibold text-terracotta">
                    Cognitive Workspace
                </span>
                <span className="ml-auto text-[8px] px-2 py-0.5 rounded-full bg-beige/30 text-charcoal font-medium">
                    Assembled
                </span>
            </div>

            {/* Query */}
            <div className="bg-cream rounded-xl p-3 mb-3 border border-terracotta/20">
                <p className="text-[9px] text-charcoal/60 mb-1">Query</p>
                <p className="text-[10px] text-terracotta font-medium">
                    &quot;What does the user do and where?&quot;
                </p>
            </div>

            {/* Retrieved context items */}
            <div className="space-y-2 mb-3">
                {[
                    { type: "Episodic", score: "0.94", tokens: "280", fill: "80%", color: "pink" },
                    { type: "Semantic", score: "0.91", tokens: "45", fill: "40%", color: "purple" },
                ].map((item) => (
                    <div key={item.type} className="flex items-center gap-3">
                        <div className="flex-1">
                            <div className="flex justify-between mb-0.5">
                                <span className="text-[9px] font-medium text-terracotta">
                                    {item.type}
                                </span>
                                <span className="text-[8px] text-charcoal/60">
                                    DIG: {item.score} · {item.tokens} tok
                                </span>
                            </div>
                            <div className="bg-cream rounded-full h-1.5 overflow-hidden">
                                <motion.div
                                    className={`h-full rounded-full ${item.color === "pink" ? "bg-terracotta/70" : "bg-terracotta"}`}
                                    initial={{ width: "0%" }}
                                    whileInView={{ width: item.fill }}
                                    viewport={{ once: true }}
                                    transition={{ duration: 1, delay: 0.3 }}
                                />
                            </div>
                        </div>
                    </div>
                ))}
            </div>

            {/* Budget gauge */}
            <div className="flex items-center justify-between bg-cream rounded-lg p-2 border border-terracotta/20">
                <span className="text-[8px] text-charcoal/60">Token Budget</span>
                <div className="flex items-center gap-2">
                    <span className="text-[9px] font-mono text-terracotta">
                        325 / 4096
                    </span>
                    <span className="text-[8px] text-charcoal font-medium">✓ optimal</span>
                </div>
            </div>
        </div>
    );
}

/* ── Steps Data ── */

const steps = [
    {
        number: "01",
        accent: "purple",
        accentBg: "bg-terracotta",
        accentLight: "bg-terracotta/20",
        accentText: "text-terracotta",
        numberColor: "text-terracotta/40",
        title: "Ingest",
        headline: "Stream conversations into episodic memory",
        description:
            "Send your user conversations, documents, or any text stream to Memora's ingest API. The surprisal segmentation engine automatically detects topic boundaries and creates meaningful episodes — no manual chunking required.",
        bullets: [
            "Auto-detects topic changes via Bayesian Surprise",
            "Extracts entity-relationship triples in real-time",
            "Stores episodes in Qdrant with rich metadata",
        ],
        mockup: <IngestMockup />,
    },
    {
        number: "02",
        accent: "pink",
        accentBg: "bg-charcoal",
        accentLight: "bg-charcoal/10",
        accentText: "text-charcoal",
        numberColor: "text-charcoal/40",
        title: "Consolidate",
        headline: "AI synthesizes and resolves knowledge",
        description:
            "Like biological sleep, CMA clusters related episodes using DBSCAN, synthesizes knowledge triples via LLM, and resolves contradictions with temporal decay. Your knowledge graph grows stronger with every cycle.",
        bullets: [
            "DBSCAN clustering finds related episodes",
            "LLM synthesizes new knowledge triples",
            "Temporal decay resolves conflicting facts",
        ],
        mockup: <ConsolidateMockup />,
    },
    {
        number: "03",
        accent: "blue",
        accentBg: "bg-beige",
        accentLight: "bg-beige/30",
        accentText: "text-charcoal",
        numberColor: "text-beige/60",
        title: "Query",
        headline: "Optimally-packed context for every question",
        description:
            "The Cognitive Workspace breaks queries into sub-questions, retrieves from both episodic and semantic memory, reranks with DIG scoring, and packs the optimal context subset within your token budget.",
        bullets: [
            "DIG reranking maximizes information gain",
            "Lagrangian knapsack optimizes token budget",
            "Dual-store retrieval (episodes + graph)",
        ],
        mockup: <WorkspaceMockup />,
    },
];

export default function StepsSection() {
    const [activeStep, setActiveStep] = useState(0);

    return (
        <section className="py-28 px-6 bg-cream relative overflow-hidden">
            {/* Background decoration */}
            <div className="absolute top-0 left-1/2 -translate-x-1/2 w-[800px] h-[800px] bg-gradient-radial from-terracotta/5 to-transparent rounded-full blur-3xl pointer-events-none" />

            <div className="max-w-6xl mx-auto relative">
                {/* Header */}
                <motion.div
                    initial={{ opacity: 0, y: 20 }}
                    whileInView={{ opacity: 1, y: 0 }}
                    viewport={{ once: true }}
                    transition={{ duration: 0.6 }}
                    className="mb-20"
                >
                    <div className="flex items-center gap-2 mb-6">
                        <div className="h-px w-8 bg-terracotta" />
                        <span className="text-xs font-semibold uppercase tracking-widest text-terracotta">
                            How it Works
                        </span>
                    </div>
                    <h2 className="text-5xl md:text-6xl font-bold text-charcoal leading-[1.1] tracking-tight">
                        Chaos to clarity
                        <br />
                        <span className="bg-gradient-to-r from-terracotta to-beige bg-clip-text text-transparent">
                            in three steps.
                        </span>
                    </h2>
                    <p className="mt-6 text-lg text-charcoal/70 leading-relaxed max-w-xl">
                        From raw conversation to optimally-assembled context — Memora
                        handles the entire memory lifecycle automatically.
                    </p>
                </motion.div>

                {/* Step selector tabs */}
                <motion.div
                    initial={{ opacity: 0, y: 20 }}
                    whileInView={{ opacity: 1, y: 0 }}
                    viewport={{ once: true }}
                    transition={{ duration: 0.6, delay: 0.1 }}
                    className="flex gap-3 mb-12"
                >
                    {steps.map((step, i) => (
                        <button
                            key={step.number}
                            onClick={() => setActiveStep(i)}
                            className={`flex-1 rounded-2xl p-5 border-2 transition-all duration-300 text-left cursor-pointer ${activeStep === i
                                ? `border-terracotta bg-terracotta text-white shadow-xl scale-[1.02]`
                                : `border-terracotta/10 bg-white text-terracotta hover:border-terracotta/30 hover:shadow-md`
                                }`}
                        >
                            <div className="flex items-center gap-3 mb-2">
                                <span
                                    className={`text-3xl font-black ${activeStep === i ? "text-white/30" : "text-terracotta/20"
                                        }`}
                                >
                                    {step.number}
                                </span>
                                <div
                                    className={`h-px flex-1 ${activeStep === i ? "bg-white/20" : "bg-cream"
                                        }`}
                                />
                            </div>
                            <h3 className="text-lg font-bold">{step.title}</h3>
                            <p
                                className={`text-xs mt-1 leading-relaxed ${activeStep === i ? "text-white/80" : "text-charcoal/60"
                                    }`}
                            >
                                {step.headline}
                            </p>
                        </button>
                    ))}
                </motion.div>

                {/* Active step content */}
                <AnimatePresence mode="wait">
                    <motion.div
                        key={activeStep}
                        initial={{ opacity: 0, y: 20 }}
                        animate={{ opacity: 1, y: 0 }}
                        exit={{ opacity: 0, y: -20 }}
                        transition={{ duration: 0.4 }}
                        className="flex flex-col lg:flex-row gap-12 items-start"
                    >
                        {/* Left: Details */}
                        <div className="lg:w-[45%] space-y-6">
                            <div>
                                <div className="flex items-center gap-3 mb-4">
                                    <div
                                        className={`w-12 h-12 rounded-2xl ${steps[activeStep].accentBg} flex items-center justify-center text-white font-black text-lg`}
                                    >
                                        {steps[activeStep].number}
                                    </div>
                                    <div>
                                        <h3 className="text-2xl font-bold text-terracotta">
                                            {steps[activeStep].title}
                                        </h3>
                                        <p className="text-xs text-charcoal/60">
                                            {steps[activeStep].headline}
                                        </p>
                                    </div>
                                </div>
                                <p className="text-sm text-charcoal/70 leading-relaxed">
                                    {steps[activeStep].description}
                                </p>
                            </div>

                            {/* Bullet points */}
                            <div className="space-y-3">
                                {steps[activeStep].bullets.map((bullet, i) => (
                                    <motion.div
                                        key={bullet}
                                        initial={{ opacity: 0, x: -10 }}
                                        animate={{ opacity: 1, x: 0 }}
                                        transition={{ delay: i * 0.1 + 0.2 }}
                                        className="flex items-start gap-3"
                                    >
                                        <div
                                            className={`w-5 h-5 rounded-lg ${steps[activeStep].accentLight} ${steps[activeStep].accentText} flex items-center justify-center text-[10px] font-bold flex-shrink-0 mt-0.5`}
                                        >
                                            ✓
                                        </div>
                                        <p className="text-sm text-charcoal/70">{bullet}</p>
                                    </motion.div>
                                ))}
                            </div>

                            {/* CTA */}
                            <div className="flex gap-3 pt-2">
                                <button className="text-xs font-semibold text-white bg-terracotta px-5 py-2.5 rounded-xl hover:bg-terracotta/90 transition-colors cursor-pointer">
                                    Read Docs →
                                </button>
                                <button className="text-xs font-semibold text-terracotta bg-cream border border-terracotta/20 px-5 py-2.5 rounded-xl hover:bg-terracotta/10 transition-colors cursor-pointer">
                                    View API
                                </button>
                            </div>
                        </div>

                        {/* Right: Mockup */}
                        <div className="lg:w-[55%]">
                            <motion.div
                                initial={{ opacity: 0, scale: 0.95 }}
                                animate={{ opacity: 1, scale: 1 }}
                                transition={{ duration: 0.5, delay: 0.1 }}
                            >
                                {steps[activeStep].mockup}
                            </motion.div>
                        </div>
                    </motion.div>
                </AnimatePresence>
            </div>
        </section>
    );
}
