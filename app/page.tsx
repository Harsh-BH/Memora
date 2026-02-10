import { DashboardPreview } from "@/components/landing/DashboardPreview";
import { FAQSection } from "@/components/landing/FAQSection";
import { FeaturesSection } from "@/components/landing/FeaturesSection";
import { Footer } from "@/components/landing/Footer";
import { HeroSection } from "@/components/landing/HeroSection";
import { LogosSection } from "@/components/landing/LogosSection";
import { MidCTASection } from "@/components/landing/MidCTASection";
import { Navbar } from "@/components/landing/Navbar";
import { TestimonialsSection } from "@/components/landing/TestimonialsSection";
import { ThreeStepsSection } from "@/components/landing/ThreeStepsSection";

export default function Home() {
  return (
    <div className="min-h-screen bg-white text-gray-900">
      <Navbar />
      <main>
        <div className="bg-white">
          <HeroSection />
        </div>
        <div className="bg-gray-50">
          <DashboardPreview />
        </div>
        <div className="bg-white">
          <LogosSection />
        </div>
        <div className="bg-gray-50">
          <FeaturesSection />
        </div>
        <div className="bg-white">
          <ThreeStepsSection />
        </div>
        <div className="bg-gray-50">
          <MidCTASection />
        </div>
        <div className="bg-white">
          <TestimonialsSection />
        </div>
        <div className="bg-gray-50">
          <FAQSection />
        </div>
      </main>
      <Footer />
    </div>
  );
}
