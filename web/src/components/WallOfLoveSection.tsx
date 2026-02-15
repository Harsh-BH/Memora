"use client";

import { motion } from "framer-motion";
import CardSwap, { Card } from "./CardSwap";

const stats = [
    { value: "15 hrs", label: "Saved/Project", color: "bg-cream border-terracotta/20" },
    { value: "96%", label: "Recall Accuracy", color: "bg-cream border-terracotta/20" },
    { value: "40%", label: "Faster Responses", color: "bg-cream border-beige/40" },
    { value: "25%", label: "Fewer Hallucinations", color: "bg-cream border-terracotta/20" },
];

export default function WallOfLoveSection() {
    return (
        <section className="py-28 px-6 overflow-visible relative bg-cream">
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
                            Testimonials
                        </span>
                    </div>

                    <div className="flex items-end justify-between gap-8">
                        <div className="max-w-2xl">
                            <h2 className="text-5xl md:text-6xl font-bold text-charcoal leading-[1.1] tracking-tight">
                                Loved by
                                <br />
                                <span className="bg-gradient-to-r from-terracotta to-beige bg-clip-text text-transparent">
                                    builders everywhere.
                                </span>
                            </h2>
                            <p className="mt-6 text-lg text-terracotta leading-relaxed max-w-xl">
                                From solo developers to enterprise teams, Memora is helping AI
                                products remember better, respond faster, and deliver
                                experiences users love.
                            </p>
                        </div>

                        <motion.div
                            initial={{ opacity: 0, y: 20 }}
                            whileInView={{ opacity: 1, y: 0 }}
                            viewport={{ once: true }}
                            transition={{ duration: 0.7, delay: 0.3 }}
                            className="hidden lg:block flex-shrink-0"
                        >
                            <img
                                src="/mascot-computer.png"
                                alt="Mascot at computer"
                                className="w-28 h-28 object-contain drop-shadow-lg"
                            />
                        </motion.div>
                    </div>
                </motion.div>

                {/* Content */}
                <div className="flex flex-col lg:flex-row items-start gap-16">
                    {/* Left: Stats + Summary */}
                    <motion.div
                        initial={{ opacity: 0, x: -30 }}
                        whileInView={{ opacity: 1, x: 0 }}
                        viewport={{ once: true }}
                        transition={{ duration: 0.6, delay: 0.2 }}
                        className="lg:w-[40%] space-y-6"
                    >
                        {/* Stats grid */}
                        <div className="grid grid-cols-2 gap-3">
                            {stats.map((s) => (
                                <div
                                    key={s.label}
                                    className={`${s.color} border rounded-xl p-4`}
                                >
                                    <p className="text-2xl font-bold text-terracotta">{s.value}</p>
                                    <p className="text-[10px] text-charcoal/60 uppercase tracking-wider mt-1">
                                        {s.label}
                                    </p>
                                </div>
                            ))}
                        </div>

                        {/* Featured quote */}
                        <div className="bg-white rounded-2xl border border-terracotta/20 p-6 shadow-sm">
                            <div className="flex items-center gap-2 mb-3">
                                <div className="flex -space-x-2">
                                    {["bg-terracotta/20", "bg-beige/30", "bg-charcoal/10", "bg-terracotta/40"].map((bg, i) => (
                                        <div key={i} className={`w-7 h-7 rounded-full ${bg} border-2 border-white`} />
                                    ))}
                                </div>
                                <span className="text-[10px] text-terracotta/60 ml-1">
                                    +240 developers
                                </span>
                            </div>
                            <p className="text-xs text-terracotta leading-relaxed">
                                Join hundreds of developers who are building AI products with
                                persistent memory. From conversational AI to personal assistants
                                — Memora powers the next generation of intelligent
                                applications.
                            </p>
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
                            cardDistance={45}
                            verticalDistance={55}
                            delay={3500}
                            pauseOnHover={true}
                            width={420}
                            height={280}
                            skewAmount={4}
                            easing="elastic"
                        >
                            <Card className="p-8 flex flex-col justify-between border border-terracotta/20 shadow-lg shadow-terracotta/5">
                                <div>
                                    <div className="flex items-center gap-3 mb-4">
                                        <div className="w-11 h-11 rounded-full bg-terracotta/10 flex items-center justify-center text-sm font-bold text-terracotta">
                                            JC
                                        </div>
                                        <div>
                                            <p className="text-sm font-bold text-charcoal">
                                                James Chen
                                            </p>
                                            <p className="text-[10px] text-charcoal/60">
                                                CTO at Xerrax · AI Platform
                                            </p>
                                        </div>
                                    </div>
                                    <p className="text-sm text-terracotta leading-relaxed">
                                        &ldquo;Before Memora, our chatbot forgot users within
                                        seconds. Now it retains context across sessions with
                                        unprecedented accuracy. Our user satisfaction scores jumped
                                        35% in the first month.&rdquo;
                                    </p>
                                </div>
                                <div className="mt-4 flex items-center justify-between">
                                    <div className="flex gap-0.5">
                                        {[1, 2, 3, 4, 5].map((i) => (
                                            <span key={i} className="text-terracotta text-sm">★</span>
                                        ))}
                                    </div>
                                    <span className="text-[10px] text-terracotta/60">
                                        15 hours saved per project
                                    </span>
                                </div>
                            </Card>

                            <Card className="p-8 flex flex-col justify-between border border-terracotta/20 shadow-lg shadow-terracotta/5">
                                <div>
                                    <div className="flex items-center gap-3 mb-4">
                                        <div className="w-11 h-11 rounded-full bg-beige/30 flex items-center justify-center text-sm font-bold text-terracotta/80">
                                            MK
                                        </div>
                                        <div>
                                            <p className="text-sm font-bold text-charcoal">
                                                Maria Koster
                                            </p>
                                            <p className="text-[10px] text-charcoal/60">
                                                Engineering Director at Rapture · EdTech
                                            </p>
                                        </div>
                                    </div>
                                    <p className="text-sm text-terracotta leading-relaxed">
                                        &ldquo;Sleep consolidation catches contradictions we never
                                        would have found manually. We used to spend hours debugging
                                        context issues — now the system handles it automatically.&rdquo;
                                    </p>
                                </div>
                                <div className="mt-4 flex items-center justify-between">
                                    <div className="flex gap-0.5">
                                        {[1, 2, 3, 4, 5].map((i) => (
                                            <span key={i} className="text-terracotta text-sm">★</span>
                                        ))}
                                    </div>
                                    <span className="text-[10px] text-terracotta/60">
                                        25% fewer hallucinations
                                    </span>
                                </div>
                            </Card>

                            <Card className="p-8 flex flex-col justify-between border border-terracotta/20 shadow-lg shadow-terracotta/5">
                                <div>
                                    <div className="flex items-center gap-3 mb-4">
                                        <div className="w-11 h-11 rounded-full bg-charcoal/10 flex items-center justify-center text-sm font-bold text-terracotta">
                                            AW
                                        </div>
                                        <div>
                                            <p className="text-sm font-bold text-charcoal">
                                                Alex Wong
                                            </p>
                                            <p className="text-[10px] text-charcoal/60">
                                                Lead Engineer at Gozer · FinTech
                                            </p>
                                        </div>
                                    </div>
                                    <p className="text-sm text-terracotta leading-relaxed">
                                        &ldquo;DIG reranking + Knapsack packing cut our context
                                        hallucinations by 25%. The cognitive workspace ensures we
                                        only send the highest-value context to the model.&rdquo;
                                    </p>
                                </div>
                                <div className="mt-4 flex items-center justify-between">
                                    <div className="flex gap-0.5">
                                        {[1, 2, 3, 4, 5].map((i) => (
                                            <span key={i} className="text-terracotta text-sm">★</span>
                                        ))}
                                    </div>
                                    <span className="text-[10px] text-terracotta/60">
                                        96% recall accuracy
                                    </span>
                                </div>
                            </Card>

                            <Card className="p-8 flex flex-col justify-between border border-terracotta/20 shadow-lg shadow-terracotta/5">
                                <div>
                                    <div className="flex items-center gap-3 mb-4">
                                        <div className="w-11 h-11 rounded-full bg-cream border border-terracotta/20 flex items-center justify-center text-sm font-bold text-terracotta">
                                            SP
                                        </div>
                                        <div>
                                            <p className="text-sm font-bold text-charcoal">
                                                Sarah Park
                                            </p>
                                            <p className="text-[10px] text-charcoal/60">
                                                VP Product at Omnicorp · Enterprise AI
                                            </p>
                                        </div>
                                    </div>
                                    <p className="text-sm text-terracotta leading-relaxed">
                                        &ldquo;40% faster response times since switching to Memora.
                                        Our enterprise clients noticed the difference immediately.
                                        The self-hosting option sealed the deal for compliance.&rdquo;
                                    </p>
                                </div>
                                <div className="mt-4 flex items-center justify-between">
                                    <div className="flex gap-0.5">
                                        {[1, 2, 3, 4, 5].map((i) => (
                                            <span key={i} className="text-terracotta text-sm">★</span>
                                        ))}
                                    </div>
                                    <span className="text-[10px] text-terracotta/60">
                                        40% faster responses
                                    </span>
                                </div>
                            </Card>
                        </CardSwap>
                    </motion.div>
                </div>
            </div>
        </section>
    );
}
