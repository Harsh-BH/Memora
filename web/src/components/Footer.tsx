"use client";

import { motion } from "framer-motion";

const navColumns = [
    {
        title: "Navigation",
        links: [
            "Features",
            "How It Works",
            "Why Use Memora?",
            "Testimonials",
            "FAQ",
        ],
    },
    {
        title: "Pages",
        links: [
            "Home",
            "Pricing",
            "Blog",
            "Contact Us",
            "Terms of Use",
            "Privacy Policy",
            "API",
        ],
    },
];

export default function Footer() {
    return (
        <footer className="relative px-6 pb-8 pt-4 bg-cream">
            {/* Botanical decoration behind footer */}
            <div className="absolute right-12 top-0 bottom-0 pointer-events-none hidden lg:flex items-center opacity-50">
                <img
                    src="/botanical.jpg"
                    alt=""
                    className="w-40 h-40 object-contain mix-blend-multiply"
                />
            </div>

            <div className="max-w-4xl mx-auto">
                {/* Links */}
                <motion.div
                    initial={{ opacity: 0, y: 20 }}
                    whileInView={{ opacity: 1, y: 0 }}
                    viewport={{ once: true }}
                    transition={{ duration: 0.6 }}
                    className="flex gap-20 py-10 border-b border-terracotta/20"
                >
                    {navColumns.map((col) => (
                        <div key={col.title}>
                            <h4 className="text-xs font-semibold text-terracotta uppercase tracking-wider mb-4">
                                {col.title}
                            </h4>
                            <ul className="space-y-2.5">
                                {col.links.map((link) => (
                                    <li key={link}>
                                        <a
                                            href="#"
                                            className="text-sm text-charcoal/60 hover:text-terracotta transition-colors"
                                        >
                                            {link}
                                        </a>
                                    </li>
                                ))}
                            </ul>
                        </div>
                    ))}

                    {/* Social Icons */}
                    <div className="ml-auto flex flex-col items-end justify-end">
                        <div className="flex gap-3">
                            {["𝕏", "◉", "◫", "in"].map((icon, i) => (
                                <a
                                    key={i}
                                    href="#"
                                    className="w-8 h-8 rounded-full bg-white border border-terracotta/10 flex items-center justify-center text-xs text-charcoal/60 hover:bg-terracotta/10 hover:text-terracotta transition-colors"
                                >
                                    {icon}
                                </a>
                            ))}
                        </div>
                    </div>
                </motion.div>

                {/* Bottom bar */}
                <motion.div
                    initial={{ opacity: 0 }}
                    whileInView={{ opacity: 1 }}
                    viewport={{ once: true }}
                    transition={{ duration: 0.6, delay: 0.2 }}
                    className="flex items-center justify-between py-6"
                >
                    {/* Logo */}
                    <div className="flex items-center gap-2">
                        <div className="w-6 h-6 rounded-full bg-gradient-to-br from-terracotta to-beige flex items-center justify-center">
                            <span className="text-[8px] text-white font-bold">M</span>
                        </div>
                        <span className="text-sm font-semibold text-charcoal">Memora</span>
                    </div>

                    {/* Credits */}
                    <div className="flex items-center gap-6 text-xs text-charcoal/60">
                        <span>Template by Samyadeep</span>
                        <span>·</span>
                        <span>Made in Next.js</span>
                        <span>·</span>
                        <span>© 2025 Memora</span>
                    </div>
                </motion.div>
            </div>
        </footer>
    );
}
