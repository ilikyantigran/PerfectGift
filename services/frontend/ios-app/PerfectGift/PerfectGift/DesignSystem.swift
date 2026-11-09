import SwiftUI

// MARK: - Amore design system
//
// Ported from the "Surprise Event App Design" Figma/React reference
// (src/index.css @theme tokens + per-screen styles). Colors, typography and the
// reusable building blocks all live here so every screen shares one look.

extension Color {
    init(hex: UInt, alpha: Double = 1) {
        self.init(
            .sRGB,
            red: Double((hex >> 16) & 0xff) / 255,
            green: Double((hex >> 8) & 0xff) / 255,
            blue: Double(hex & 0xff) / 255,
            opacity: alpha
        )
    }
}

enum Theme {
    static let canvas   = Color(hex: 0x07040a)
    static let surface  = Color(hex: 0x1a1020)
    static let elevated = Color(hex: 0x251729)
    static let rose     = Color(hex: 0xd4778a)
    static let roseDeep = Color(hex: 0xb85a70)
    static let peach    = Color(hex: 0xe8a07a)
    static let cream    = Color(hex: 0xf5ebe8)
    static let muted    = Color(hex: 0xa48a96)
    static let border   = Color(hex: 0x2e1b2a)
    /// Softer body text used inside idea cards.
    static let softText = Color(hex: 0xc8b0bc)

    static let ctaGradient = LinearGradient(
        colors: [rose, roseDeep],
        startPoint: .topLeading, endPoint: .bottomTrailing
    )
    static let warmGradient = LinearGradient(
        colors: [peach, rose],
        startPoint: .topLeading, endPoint: .bottomTrailing
    )
}

extension Font {
    /// Playfair Display stand-in — iOS "New York" serif.
    static func display(_ size: CGFloat, weight: Font.Weight = .bold) -> Font {
        .system(size: size, weight: weight, design: .serif)
    }
    /// DM Sans stand-in — system sans.
    static func amoreBody(_ size: CGFloat, weight: Font.Weight = .regular) -> Font {
        .system(size: size, weight: weight, design: .default)
    }
}

// MARK: - Background

/// The dark plum vertical gradient shared by every screen, plus ambient rose/peach glows.
struct ScreenBackground: View {
    var glows = true
    var body: some View {
        ZStack {
            LinearGradient(
                colors: [Color(hex: 0x130d18), Color(hex: 0x0d0612), Theme.canvas],
                startPoint: .top, endPoint: .bottom
            )
            if glows {
                GeometryReader { geo in
                    ZStack {
                        Circle()
                            .fill(RadialGradient(colors: [Theme.rose.opacity(0.12), .clear],
                                                 center: .center, startRadius: 0, endRadius: 300))
                            .frame(width: 600, height: 600)
                            .position(x: geo.size.width * 0.5, y: geo.size.height * 0.18)
                        Circle()
                            .fill(RadialGradient(colors: [Theme.peach.opacity(0.08), .clear],
                                                 center: .center, startRadius: 0, endRadius: 200))
                            .frame(width: 400, height: 400)
                            .position(x: geo.size.width * 0.78, y: geo.size.height * 0.85)
                    }
                }
                .allowsHitTesting(false)
            }
        }
        .ignoresSafeArea()
    }
}

// MARK: - Small building blocks

/// Uppercase, wide-tracked field label ("EMAIL", "OCCASION"…).
struct FieldLabel: View {
    let text: String
    var body: some View {
        Text(text.uppercased())
            .font(.amoreBody(11))
            .tracking(1.2)
            .foregroundStyle(Theme.muted)
            .frame(maxWidth: .infinity, alignment: .leading)
    }
}

/// A back chevron in a rounded surface tile — the design's screen header button.
struct BackButton: View {
    let action: () -> Void
    var body: some View {
        Button(action: action) {
            Image(systemName: "chevron.left")
                .font(.system(size: 15, weight: .semibold))
                .foregroundStyle(Theme.muted)
                .frame(width: 36, height: 36)
                .background(RoundedRectangle(cornerRadius: 12, style: .continuous).fill(Theme.surface))
                .overlay(RoundedRectangle(cornerRadius: 12, style: .continuous).strokeBorder(Theme.border, lineWidth: 1))
        }
    }
}

/// Primary rose CTA button, used across screens.
struct PrimaryButton: View {
    let title: String
    var icon: String? = nil
    var enabled: Bool = true
    let action: () -> Void

    var body: some View {
        Button(action: action) {
            HStack(spacing: 8) {
                if let icon { Image(systemName: icon).font(.system(size: 16, weight: .semibold)) }
                Text(title).font(.amoreBody(16, weight: .semibold))
            }
            .foregroundStyle(enabled ? Theme.cream : Theme.muted)
            .frame(maxWidth: .infinity)
            .padding(.vertical, 16)
            .background {
                if enabled {
                    RoundedRectangle(cornerRadius: 18, style: .continuous)
                        .fill(Theme.ctaGradient)
                        .shadow(color: Theme.rose.opacity(0.4), radius: 18, y: 10)
                } else {
                    RoundedRectangle(cornerRadius: 18, style: .continuous).fill(Theme.surface)
                }
            }
        }
        .buttonStyle(PressableStyle())
        .disabled(!enabled)
    }
}

/// Secondary/outline button (e.g. "Generate more ideas").
struct SecondaryButton: View {
    let title: String
    var icon: String? = nil
    let action: () -> Void
    var body: some View {
        Button(action: action) {
            HStack(spacing: 6) {
                if let icon { Image(systemName: icon).font(.system(size: 14)) }
                Text(title).font(.amoreBody(14, weight: .medium))
            }
            .foregroundStyle(Theme.muted)
            .frame(maxWidth: .infinity)
            .padding(.vertical, 14)
            .background(RoundedRectangle(cornerRadius: 18, style: .continuous).fill(Theme.surface))
            .overlay(RoundedRectangle(cornerRadius: 18, style: .continuous).strokeBorder(Theme.border, lineWidth: 1))
        }
        .buttonStyle(PressableStyle())
    }
}

/// Subtle scale-on-press feedback (mirrors the web onMouseDown scale).
struct PressableStyle: ButtonStyle {
    func makeBody(configuration: Configuration) -> some View {
        configuration.label
            .scaleEffect(configuration.isPressed ? 0.98 : 1)
            .animation(.easeOut(duration: 0.12), value: configuration.isPressed)
    }
}

extension View {
    /// Surface card fill + border used on inputs and pickers.
    func surfaceCard(cornerRadius: CGFloat = 18, active: Bool = false) -> some View {
        background(RoundedRectangle(cornerRadius: cornerRadius, style: .continuous).fill(Theme.surface))
            .overlay(RoundedRectangle(cornerRadius: cornerRadius, style: .continuous)
                .strokeBorder(active ? Theme.rose.opacity(0.5) : Theme.border, lineWidth: 1))
    }
}

/// Themed multiline text field matching the design's dark surface inputs.
struct AmoreTextEditor: View {
    let placeholder: String
    @Binding var text: String
    var height: CGFloat = 96
    var body: some View {
        ZStack(alignment: .topLeading) {
            if text.isEmpty {
                Text(placeholder)
                    .font(.amoreBody(14))
                    .foregroundStyle(Theme.muted.opacity(0.5))
                    .padding(.horizontal, 16).padding(.vertical, 14)
            }
            TextEditor(text: $text)
                .font(.amoreBody(14))
                .foregroundStyle(Theme.cream)
                .scrollContentBackground(.hidden)
                .frame(height: height)
                .padding(.horizontal, 12).padding(.vertical, 7)
        }
        .surfaceCard()
    }
}
