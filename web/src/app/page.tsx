import Navbar from "@/components/Navbar";
import HeroSection from "@/components/HeroSection";
import DashboardMockup from "@/components/DashboardMockup";
import FeaturesSection from "@/components/FeaturesSection";
import QuoteSection from "@/components/QuoteSection";
import StepsSection from "@/components/StepsSection";
import WhyChooseSection from "@/components/WhyChooseSection";
import WallOfLoveSection from "@/components/WallOfLoveSection";
import FAQSection from "@/components/FAQSection";
import CTABanner from "@/components/CTABanner";
import Footer from "@/components/Footer";

export default function Home() {
  return (
    <main className="min-h-screen bg-[#FAF8ED] overflow-hidden">
      <Navbar />
      <HeroSection />
      <DashboardMockup />
      <FeaturesSection />
      <QuoteSection />
      <StepsSection />
      <WhyChooseSection />
      <WallOfLoveSection />
      <FAQSection />
      <CTABanner />
      <Footer />
    </main>
  );
}
