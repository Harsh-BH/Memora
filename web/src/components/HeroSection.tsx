"use client";

import { motion } from "framer-motion";

export default function HeroSection() {
    return (
        <section className="pt-32 pb-8 text-center px-6">
            <motion.h1
                initial={{ opacity: 0, y: 30 }}
                animate={{ opacity: 1, y: 0 }}
                transition={{ duration: 0.7, delay: 0.2, ease: "easeOut" as const }}
                className="text-5xl md:text-6xl lg:text-[4.2rem] font-bold text-charcoal leading-[1.1] max-w-3xl mx-auto tracking-tight"
            >
                Work smarter with AI-
                <br />
                powered project flows
            </motion.h1>

            <motion.p
                initial={{ opacity: 0, y: 20 }}
                animate={{ opacity: 1, y: 0 }}
                transition={{ duration: 0.6, delay: 0.4, ease: "easeOut" as const }}
                className="mt-6 text-charcoal/80 text-base md:text-lg max-w-xl mx-auto leading-relaxed"
            >
                Flowgenix transforms project chaos into structured clarity with smart
                workflows, predictive timelines and task intelligence.
            </motion.p>

            <motion.div
                initial={{ opacity: 0, y: 20 }}
                animate={{ opacity: 1, y: 0 }}
                transition={{ duration: 0.6, delay: 0.6, ease: "easeOut" as const }}
                className="mt-8 flex items-center justify-center gap-4"
            >
                <motion.button
                    whileHover={{ scale: 1.05 }}
                    whileTap={{ scale: 0.95 }}
                    className="bg-terracotta text-white px-6 py-3 rounded-lg text-sm font-medium hover:bg-terracotta/90 transition-colors duration-200 shadow-lg shadow-terracotta/20 cursor-pointer"
                >
                    Start Building
                </motion.button>
                <motion.button
                    whileHover={{ scale: 1.05 }}
                    whileTap={{ scale: 0.95 }}
                    className="bg-cream text-terracotta px-6 py-3 rounded-lg text-sm font-medium border border-terracotta/30 hover:border-terracotta hover:bg-terracotta/10 transition-all duration-200 cursor-pointer"
                >
                    Book Demo
                </motion.button>
            </motion.div>
        </section>
    );
}
