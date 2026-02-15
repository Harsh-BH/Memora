"use client";

import { motion } from "framer-motion";

export default function FooterBadges() {
    return (
        <motion.div
            initial={{ opacity: 0, y: 20 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ duration: 0.6, delay: 1.2, ease: "easeOut" }}
            className="fixed bottom-4 right-4 z-50 flex flex-col items-end gap-2"
        >
            <button className="bg-purple-600 text-white text-xs px-4 py-2 rounded-lg font-medium hover:bg-purple-700 transition-colors shadow-lg shadow-purple-200 cursor-pointer">
                🎨 Get Template BIG
            </button>
            <button className="bg-white text-gray-700 text-xs px-4 py-2 rounded-lg font-medium border border-gray-200 hover:bg-gray-50 transition-colors shadow-md cursor-pointer flex items-center gap-1.5">
                <span>✦</span> See All Templates
            </button>
            <div className="flex items-center gap-1.5 text-[10px] text-gray-400 bg-white/80 backdrop-blur-sm px-3 py-1.5 rounded-full border border-gray-100">
                <svg width="10" height="10" viewBox="0 0 24 24" fill="currentColor">
                    <path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm-1 17.93c-3.95-.49-7-3.85-7-7.93 0-.62.08-1.21.21-1.79L9 15v1c0 1.1.9 2 2 2v1.93zm6.9-2.54c-.26-.81-1-1.39-1.9-1.39h-1v-3c0-.55-.45-1-1-1H8v-2h2c.55 0 1-.45 1-1V7h2c1.1 0 2-.9 2-2v-.41c2.93 1.19 5 4.06 5 7.41 0 2.08-.8 3.97-2.1 5.39z" />
                </svg>
                Made in Framer
            </div>
        </motion.div>
    );
}
