"use client";

import { motion } from "framer-motion";
import ScrollReveal from "./ScrollReveal";

export default function QuoteSection() {
    return (
        <section className="py-36 px-6">
            <div className="max-w-6xl mx-auto flex flex-col md:flex-row items-center gap-14">
                {/* Illustration */}
                <motion.div
                    initial={{ opacity: 0, x: -40 }}
                    whileInView={{ opacity: 1, x: 0 }}
                    viewport={{ once: true }}
                    transition={{ duration: 0.7, ease: "easeOut" as const }}
                    className="flex-shrink-0"
                >
                    <img
                        src="/team-illustration.png"
                        alt="Team collaboration illustration"
                        className="w-96 h-96 object-contain"
                    />
                </motion.div>

                {/* ScrollReveal quote */}
                <div className="flex-1">
                    <ScrollReveal
                        baseOpacity={0.1}
                        enableBlur={true}
                        baseRotation={3}
                        blurStrength={4}
                        textClassName="text-terracotta !text-[clamp(1.4rem,2.8vw,2.2rem)] !leading-[1.7]"
                    >
                        Take your AI from "I don't remember" to "I already know" — all in a few API calls. CMA connects conversations into lasting knowledge, so your AI can achieve more with every interaction.
                    </ScrollReveal>
                </div>
            </div>
        </section>
    );
}
