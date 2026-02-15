"use client";

import { motion } from "framer-motion";
import CardSwap, { Card } from "./CardSwap";

export default function WhyChooseSection() {
    return (
        <section className="py-28 px-6 overflow-visible relative bg-cream">
            {/* <div className="absolute inset-0 bg-[#FDF4F5] pointer-events-none" /> - Removed implicit bg, using section bg-cream */}

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
                        <div className="h-px w-8 bg-terracotta/50" />
                        <span className="text-xs font-semibold uppercase tracking-widest text-terracotta">
                            Why Memora
                        </span>
                    </div>

                    <div className="flex items-end justify-between gap-8">
                        <div className="max-w-2xl">
                            <h2 className="text-5xl md:text-6xl font-bold text-charcoal leading-[1.1] tracking-tight">
                                Built different.
                                <br />
                                <span className="bg-gradient-to-r from-terracotta to-beige bg-clip-text text-transparent">
                                    Built to last.
                                </span>
                            </h2>
                            <p className="mt-6 text-lg text-charcoal/60 leading-relaxed max-w-xl">
                                Most memory solutions are glorified vector stores. Memora is a
                                full cognitive architecture — with consolidation, conflict
                                resolution, and information-theoretic retrieval that actually
                                understands what matters.
                            </p>
                        </div>

                        <motion.div
                            initial={{ opacity: 0, scale: 0.8 }}
                            whileInView={{ opacity: 1, scale: 1 }}
                            viewport={{ once: true }}
                            transition={{ duration: 0.7, delay: 0.3 }}
                            className="hidden lg:block flex-shrink-0"
                        >
                            <img
                                src="/mascot-running.png"
                                alt="Running mascot"
                                className="w-32 h-32 object-contain drop-shadow-lg"
                            />
                        </motion.div>
                    </div>
                </motion.div>

                {/* Two-column layout */}
                <div className="flex flex-col lg:flex-row items-start gap-16">
                    {/* Left: Rich info panel */}
                    <motion.div
                        initial={{ opacity: 0, x: -30 }}
                        whileInView={{ opacity: 1, x: 0 }}
                        viewport={{ once: true }}
                        transition={{ duration: 0.6, delay: 0.2 }}
                        className="lg:w-[40%] space-y-6"
                    >
                        <div>
                            <h3 className="text-2xl font-bold text-charcoal mb-3">
                                Not just RAG
                            </h3>
                            <p className="text-sm text-charcoal/60 leading-relaxed">
                                Traditional RAG systems retrieve documents and stuff them into
                                context. They don&apos;t <em>learn</em>. They don&apos;t{" "}
                                <em>consolidate</em>. They don&apos;t resolve contradictions or
                                prioritize by information gain.
                            </p>
                            <p className="text-sm text-charcoal/60 leading-relaxed mt-3">
                                Memora does all of that — and serves it through a clean REST
                                API that any LLM pipeline can consume.
                            </p>
                        </div>

                        {/* Comparison table */}
                        <div className="bg-white rounded-2xl border border-terracotta/20 overflow-hidden shadow-sm">
                            <div className="grid grid-cols-3 text-[10px] font-semibold uppercase tracking-wider text-charcoal/60 bg-cream px-4 py-2.5 border-b border-terracotta/20">
                                <span>Feature</span>
                                <span className="text-center">RAG</span>
                                <span className="text-center text-terracotta">Memora</span>
                            </div>
                            {[
                                ["Episodic Memory", "✗", "✓"],
                                ["Knowledge Graph", "✗", "✓"],
                                ["Sleep Cycles", "✗", "✓"],
                                ["Conflict Resolution", "✗", "✓"],
                                ["DIG Reranking", "✗", "✓"],
                                ["Token Optimization", "~", "✓"],
                            ].map(([feature, rag, memora]) => (
                                <div
                                    key={feature}
                                    className="grid grid-cols-3 text-xs px-4 py-2 border-b border-terracotta/10 last:border-0"
                                >
                                    <span className="text-terracotta">{feature}</span>
                                    <span className="text-center text-charcoal/40">{rag}</span>
                                    <span className="text-center text-terracotta font-semibold">
                                        {memora}
                                    </span>
                                </div>
                            ))}
                        </div>
                    </motion.div>

                    {/* Right: CardSwap */}
                    <motion.div
                        initial={{ opacity: 0 }}
                        whileInView={{ opacity: 1 }}
                        viewport={{ once: true }}
                        transition={{ duration: 0.6, delay: 0.3 }}
                        className="lg:w-[60%] relative overflow-visible"
                        style={{ height: "500px" }}
                    >
                        <CardSwap
                            cardDistance={50}
                            verticalDistance={60}
                            delay={5000}
                            pauseOnHover={true}
                            width={400}
                            height={280}
                            skewAmount={5}
                            easing="elastic"
                        >
                            <Card className="p-8 flex flex-col justify-between border border-terracotta/20 shadow-lg shadow-terracotta/5">
                                <div>
                                    <div className="flex items-center gap-3 mb-4">
                                        <span className="flex h-11 w-11 items-center justify-center rounded-xl bg-terracotta/10 text-xl">
                                            ⚡
                                        </span>
                                        <div>
                                            <h3 className="text-lg font-black text-terracotta">
                                                Built for Speed
                                            </h3>
                                            <p className="text-[10px] text-charcoal/60">
                                                sub-50ms query latency at scale
                                            </p>
                                        </div>
                                    </div>
                                    <p className="text-sm text-charcoal/70 leading-relaxed">
                                        Ingest episodes, query memories, or run consolidation in
                                        milliseconds. Qdrant vector search + Neo4j graph traversal
                                        both run concurrently for maximum throughput.
                                    </p>
                                </div>
                            </Card>

                            <Card className="p-8 flex flex-col justify-between border border-terracotta/20 shadow-lg shadow-terracotta/5">
                                <div>
                                    <div className="flex items-center gap-3 mb-4">
                                        <span className="flex h-11 w-11 items-center justify-center rounded-xl bg-beige/30 text-xl">
                                            🧠
                                        </span>
                                        <div>
                                            <h3 className="text-lg font-black text-terracotta">
                                                Deep Context Understanding
                                            </h3>
                                            <p className="text-[10px] text-charcoal/60">
                                                beyond keyword matching
                                            </p>
                                        </div>
                                    </div>
                                    <p className="text-sm text-charcoal/70 leading-relaxed">
                                        Whether it&apos;s a fleeting mention or a life-changing
                                        update, CMA captures the full depth — connecting episodes,
                                        entity triples, and temporal signals.
                                    </p>
                                </div>
                            </Card>

                            <Card className="p-8 flex flex-col justify-between border border-terracotta/20 shadow-lg shadow-terracotta/5">
                                <div>
                                    <div className="flex items-center gap-3 mb-4">
                                        <span className="flex h-11 w-11 items-center justify-center rounded-xl bg-charcoal/5 text-xl">
                                            🔄
                                        </span>
                                        <div>
                                            <h3 className="text-lg font-black text-terracotta">
                                                Zero Overhead
                                            </h3>
                                            <p className="text-[10px] text-charcoal/60">
                                                fire-and-forget API design
                                            </p>
                                        </div>
                                    </div>
                                    <p className="text-sm text-charcoal/70 leading-relaxed">
                                        Skip manual context stitching. Send conversations to the
                                        ingest endpoint and query when needed. The system handles
                                        segmentation, consolidation, and optimization automatically.
                                    </p>
                                </div>
                            </Card>

                            <Card className="p-8 flex flex-col justify-between border border-terracotta/20 shadow-lg shadow-terracotta/5">
                                <div>
                                    <div className="flex items-center gap-3 mb-4">
                                        <span className="flex h-11 w-11 items-center justify-center rounded-xl bg-cream border border-terracotta/20 text-xl">
                                            🔓
                                        </span>
                                        <div>
                                            <h3 className="text-lg font-black text-terracotta">
                                                Open Source Forever
                                            </h3>
                                            <p className="text-[10px] text-charcoal/60">
                                                self-host · no vendor lock-in
                                            </p>
                                        </div>
                                    </div>
                                    <p className="text-sm text-charcoal/70 leading-relaxed">
                                        Full control over your data and infrastructure. Self-host
                                        with Docker Compose in under 5 minutes. Multi-tenant by
                                        design with complete data isolation.
                                    </p>
                                </div>
                            </Card>
                        </CardSwap>
                    </motion.div>
                </div>
            </div>
        </section>
    );
}
