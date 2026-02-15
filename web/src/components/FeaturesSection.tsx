"use client";

import { motion } from "framer-motion";
import CardSwap, { Card } from "./CardSwap";

const stats = [
    { value: "96%", label: "Recall Accuracy" },
    { value: "<50ms", label: "Query Latency" },
    { value: "5 min", label: "Setup Time" },
];

export default function FeaturesSection() {
    return (
        <section className="py-28 px-6 overflow-visible relative">
            {/* Subtle background gradient */}
            <div className="absolute inset-0 bg-gradient-to-b from-white via-purple-50/30 to-white pointer-events-none" />

            <div className="max-w-6xl mx-auto relative">
                {/* Top: Section badge + Heading */}
                <motion.div
                    initial={{ opacity: 0, y: 20 }}
                    whileInView={{ opacity: 1, y: 0 }}
                    viewport={{ once: true }}
                    transition={{ duration: 0.6 }}
                    className="mb-20"
                >
                    {/* Badge */}
                    <div className="flex items-center gap-2 mb-6">
                        <div className="h-px w-8 bg-terracotta/50" />
                        <span className="text-xs font-semibold uppercase tracking-widest text-terracotta">
                            Core Architecture
                        </span>
                    </div>

                    {/* Heading row with mascot */}
                    <div className="flex items-end justify-between gap-8">
                        <div className="max-w-2xl">
                            <h2 className="text-5xl md:text-6xl font-bold text-charcoal leading-[1.1] tracking-tight">
                                Five modules.
                                <br />
                                <span className="bg-gradient-to-r from-terracotta to-beige bg-clip-text text-transparent">
                                    One unified memory.
                                </span>
                            </h2>
                            <p className="mt-6 text-lg text-charcoal/70 leading-relaxed max-w-xl">
                                Inspired by how the human brain encodes, consolidates, and
                                retrieves memories — Memora implements each cognitive stage as a
                                composable API module.
                            </p>
                        </div>

                        {/* Mascot floating next to heading */}
                        <motion.div
                            initial={{ opacity: 0, y: 20, rotate: -5 }}
                            whileInView={{ opacity: 1, y: 0, rotate: 0 }}
                            viewport={{ once: true }}
                            transition={{ duration: 0.7, delay: 0.3 }}
                            className="hidden lg:block flex-shrink-0"
                        >
                            <img
                                src="/mascot.png"
                                alt="Memora mascot"
                                className="w-72 h-72 object-contain drop-shadow-lg"
                            />
                        </motion.div>
                    </div>
                </motion.div>

                {/* Main content: Rich left panel + CardSwap right */}
                <div className="flex flex-col lg:flex-row items-start gap-16">
                    {/* Left: Rich info panel */}
                    <motion.div
                        initial={{ opacity: 0, x: -30 }}
                        whileInView={{ opacity: 1, x: 0 }}
                        viewport={{ once: true }}
                        transition={{ duration: 0.6, delay: 0.2 }}
                        className="lg:w-[40%] space-y-8"
                    >
                        {/* Description */}
                        <div>
                            <h3 className="text-2xl font-bold text-charcoal mb-3">
                                From Ingestion to Recall
                            </h3>
                            <p className="text-sm text-charcoal/70 leading-relaxed">
                                When a conversation enters Memora, it flows through a
                                biologically-inspired pipeline: the{" "}
                                <strong className="text-terracotta">hippocampus</strong> encodes
                                raw episodes, the{" "}
                                <strong className="text-terracotta">neocortex</strong> extracts
                                semantic knowledge, and a{" "}
                                <strong className="text-terracotta">sleep-like cycle</strong>{" "}
                                consolidates everything into a coherent knowledge graph.
                            </p>
                            <p className="text-sm text-charcoal/70 leading-relaxed mt-3">
                                At query time, the{" "}
                                <strong className="text-terracotta">Cognitive Workspace</strong>{" "}
                                optimally packs the most relevant memories into your token
                                budget using DIG reranking and Lagrangian optimization.
                            </p>
                        </div>

                        {/* Mini stat row */}
                        <div className="flex gap-4">
                            {stats.map((s) => (
                                <div
                                    key={s.label}
                                    className="flex-1 bg-white rounded-xl border border-terracotta/20 p-4 text-center shadow-sm"
                                >
                                    <p className="text-xl font-bold text-terracotta">{s.value}</p>
                                    <p className="text-[10px] text-charcoal/60 uppercase tracking-wider mt-1">
                                        {s.label}
                                    </p>
                                </div>
                            ))}
                        </div>

                        {/* Pipeline diagram */}
                        <div className="bg-cream rounded-2xl border border-terracotta/20 p-5">
                            <p className="text-[10px] uppercase tracking-widest text-terracotta font-semibold mb-3">
                                Memory Pipeline
                            </p>
                            <div className="flex items-center gap-2 text-xs">
                                <span className="bg-beige/30 text-charcoal font-medium px-3 py-1.5 rounded-lg">
                                    Ingest
                                </span>
                                <span className="text-terracotta/50">→</span>
                                <span className="bg-terracotta/20 text-terracotta font-medium px-3 py-1.5 rounded-lg">
                                    Segment
                                </span>
                                <span className="text-terracotta/50">→</span>
                                <span className="bg-charcoal/10 text-charcoal font-medium px-3 py-1.5 rounded-lg">
                                    Store
                                </span>
                                <span className="text-terracotta/50">→</span>
                                <span className="bg-cream border border-terracotta/30 text-terracotta font-medium px-3 py-1.5 rounded-lg">
                                    Sleep
                                </span>
                                <span className="text-terracotta/50">→</span>
                                <span className="bg-beige text-charcoal font-medium px-3 py-1.5 rounded-lg">
                                    Query
                                </span>
                            </div>
                        </div>
                    </motion.div>

                    {/* Right: CardSwap — overflows the section */}
                    <motion.div
                        initial={{ opacity: 0 }}
                        whileInView={{ opacity: 1 }}
                        viewport={{ once: true }}
                        transition={{ duration: 0.6, delay: 0.3 }}
                        className="lg:w-[60%] relative overflow-visible"
                        style={{ height: "520px" }}
                    >
                        <CardSwap
                            cardDistance={50}
                            verticalDistance={60}
                            delay={4000}
                            pauseOnHover={true}
                            width={420}
                            height={320}
                            skewAmount={4}
                            easing="elastic"
                        >
                            <Card className="p-8 flex flex-col justify-between bg-white border border-terracotta/20 shadow-lg shadow-terracotta/5">
                                <div>
                                    <div className="flex items-center gap-3 mb-4">
                                        <span className="flex h-11 w-11 items-center justify-center rounded-xl bg-beige/30 text-xl">
                                            📊
                                        </span>
                                        <div>
                                            <h3 className="text-lg font-black text-terracotta">
                                                Surprisal Segmentation
                                            </h3>
                                            <p className="text-[10px] text-charcoal/60">
                                                hippocampus · episode boundary detection
                                            </p>
                                        </div>
                                    </div>
                                    <p className="text-sm text-charcoal/80 leading-relaxed">
                                        Uses Bayesian Surprise to detect topic boundaries in
                                        conversation streams, creating meaningful episodic memories
                                        automatically. Each episode captures a coherent thought
                                        with full context.
                                    </p>
                                </div>
                                <div className="mt-5 text-[10px] text-charcoal font-mono bg-beige/30 rounded-lg px-3 py-2 border border-beige/40">
                                    γ = 1.5 · rolling_window(10) · S &gt; μ + γσ → new_episode()
                                </div>
                            </Card>

                            <Card className="p-8 flex flex-col justify-between bg-white border border-terracotta/20 shadow-lg shadow-terracotta/5">
                                <div>
                                    <div className="flex items-center gap-3 mb-4">
                                        <span className="flex h-11 w-11 items-center justify-center rounded-xl bg-terracotta/10 text-xl">
                                            ⬡
                                        </span>
                                        <div>
                                            <h3 className="text-lg font-black text-terracotta">
                                                Knowledge Graph
                                            </h3>
                                            <p className="text-[10px] text-charcoal/60">
                                                neocortex · semantic triple extraction
                                            </p>
                                        </div>
                                    </div>
                                    <p className="text-sm text-charcoal/80 leading-relaxed">
                                        Extracts entity-relationship triples into Neo4j, building
                                        a growing semantic neocortex. Each triple carries a
                                        confidence score and temporal metadata for decay.
                                    </p>
                                </div>
                                <div className="mt-5 text-[10px] text-terracotta font-mono bg-terracotta/10 rounded-lg px-3 py-2 border border-terracotta/20">
                                    (User, works_at, Google) → conf: 0.98 · decay: 0.95/cycle
                                </div>
                            </Card>

                            <Card className="p-8 flex flex-col justify-between bg-white border border-terracotta/20 shadow-lg shadow-terracotta/5">
                                <div>
                                    <div className="flex items-center gap-3 mb-4">
                                        <span className="flex h-11 w-11 items-center justify-center rounded-xl bg-charcoal/10 text-xl">
                                            🌙
                                        </span>
                                        <div>
                                            <h3 className="text-lg font-black text-terracotta">
                                                Sleep Consolidation
                                            </h3>
                                            <p className="text-[10px] text-charcoal/60">
                                                neocortex · DBSCAN clustering · conflict resolution
                                            </p>
                                        </div>
                                    </div>
                                    <p className="text-sm text-charcoal/80 leading-relaxed">
                                        Just like biological sleep, CMA clusters related episodes
                                        via DBSCAN, synthesizes knowledge triples using an LLM,
                                        and resolves contradictions with temporal decay.
                                    </p>
                                </div>
                                <div className="mt-5 text-[10px] text-terracotta font-mono bg-charcoal/10 rounded-lg px-3 py-2 border border-charcoal/20">
                                    DBSCAN(eps=0.3, min=3) → synthesize → resolve_conflicts()
                                </div>
                            </Card>

                            <Card className="p-8 flex flex-col justify-between bg-white border border-terracotta/20 shadow-lg shadow-terracotta/5">
                                <div>
                                    <div className="flex items-center gap-3 mb-4">
                                        <span className="flex h-11 w-11 items-center justify-center rounded-xl bg-beige/30 text-xl">
                                            ⚡
                                        </span>
                                        <div>
                                            <h3 className="text-lg font-black text-terracotta">
                                                Cognitive Workspace
                                            </h3>
                                            <p className="text-[10px] text-charcoal/60">
                                                working memory · DIG + Knapsack optimization
                                            </p>
                                        </div>
                                    </div>
                                    <p className="text-sm text-charcoal/80 leading-relaxed">
                                        Breaks queries into sub-questions, retrieves candidates
                                        from both stores, reranks with DIG scoring, and packs
                                        the optimal subset within your token budget.
                                    </p>
                                </div>
                                <div className="mt-5 text-[10px] text-charcoal font-mono bg-beige/30 rounded-lg px-3 py-2 border border-beige/40">
                                    DIG(q) → rank → knapsack(λ*=0.42, budget=4096) → context
                                </div>
                            </Card>

                            <Card className="p-8 flex flex-col justify-between bg-white border border-terracotta/20 shadow-lg shadow-terracotta/5">
                                <div>
                                    <div className="flex items-center gap-3 mb-4">
                                        <span className="flex h-11 w-11 items-center justify-center rounded-xl bg-terracotta/10 text-xl">
                                            🧠
                                        </span>
                                        <div>
                                            <h3 className="text-lg font-black text-terracotta">
                                                Episodic Memory
                                            </h3>
                                            <p className="text-[10px] text-charcoal/60">
                                                hippocampus · Qdrant vector store
                                            </p>
                                        </div>
                                    </div>
                                    <p className="text-sm text-charcoal/80 leading-relaxed">
                                        Stores conversation episodes in Qdrant with rich metadata
                                        — timestamps, emotional valence, entity references, and
                                        cross-links for precise semantic recall.
                                    </p>
                                </div>
                                <div className="mt-5 text-[10px] text-terracotta font-mono bg-terracotta/10 rounded-lg px-3 py-2 border border-terracotta/20">
                                    qdrant.upsert(embedding, meta{`{ts, entities, emotion}`})
                                </div>
                            </Card>
                        </CardSwap>
                    </motion.div>
                </div>
            </div>
        </section>
    );
}
