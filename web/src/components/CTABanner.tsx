"use client";

import { motion } from "framer-motion";

export default function CTABanner() {
    return (
        <section className="py-12 px-6 bg-cream">
            <div className="max-w-4xl mx-auto">
                <motion.div
                    initial={{ opacity: 0, y: 30 }}
                    whileInView={{ opacity: 1, y: 0 }}
                    viewport={{ once: true }}
                    transition={{ duration: 0.7 }}
                    className="bg-terracotta/10 border border-terracotta/20 rounded-3xl p-10 md:p-14 relative overflow-hidden"
                >
                    <div className="flex items-center justify-between">
                        {/* Left text */}
                        <div className="max-w-md relative z-10">
                            <h2 className="text-3xl md:text-4xl font-bold text-charcoal leading-tight italic">
                                Remember <span className="not-italic">🧠</span> smarter
                                <br />
                                Recall <span className="not-italic">🚀</span> faster
                            </h2>
                            <p className="mt-4 text-sm text-charcoal/70 leading-relaxed">
                                Let CMA build, manage, and optimize your AI&apos;s memory — so your
                                models can do their best work, without the forgetfulness.
                            </p>
                            <motion.button
                                whileHover={{ scale: 1.05 }}
                                whileTap={{ scale: 0.95 }}
                                className="mt-6 bg-terracotta text-white text-sm px-6 py-3 rounded-lg font-medium hover:bg-terracotta/90 transition-colors cursor-pointer"
                            >
                                Start Building Your Memory
                            </motion.button>
                        </div>

                        {/* Mascot */}
                        <div className="hidden md:block flex-shrink-0">
                            <img
                                src="/mascot-superhero.png"
                                alt="AI Superhero mascot"
                                className="w-44 h-44 object-contain"
                            />
                        </div>
                    </div>
                </motion.div>
            </div>
        </section>
    );
}
