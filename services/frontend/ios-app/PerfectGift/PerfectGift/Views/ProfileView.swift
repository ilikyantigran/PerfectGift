import SwiftUI
import PerfectGiftKit

/// Profile (Amore design): avatar + identity, a decorative stats row, and the two actions
/// the existing app exposed via the toolbar menu — "Generate Poll" and "Sign Out".
struct ProfileView: View {
    let email: String?
    let displayName: String?
    let onGeneratePoll: () -> Void
    let onSignOut: () -> Void
    let onBack: () -> Void

    private var name: String { displayName ?? "Alex" }
    private var initial: String { String(name.first ?? "A") }

    var body: some View {
        ZStack {
            ScreenBackground()
            ScrollView(showsIndicators: false) {
                VStack(spacing: 0) {
                    header
                    avatarCard
                    menu
                }
                .padding(.bottom, 32)
            }
        }
    }

    private var header: some View {
        HStack(spacing: 12) {
            BackButton(action: onBack)
            Text("Profile").font(.display(20, weight: .semibold)).foregroundStyle(Theme.cream)
            Spacer()
        }
        .padding(.horizontal, 24).padding(.top, 12).padding(.bottom, 20)
    }

    private var avatarCard: some View {
        VStack(spacing: 0) {
            Circle()
                .fill(LinearGradient(colors: [Theme.rose.opacity(0.35), Theme.peach.opacity(0.2)],
                                     startPoint: .topLeading, endPoint: .bottomTrailing))
                .frame(width: 84, height: 84)
                .overlay(Circle().strokeBorder(Theme.rose.opacity(0.4), lineWidth: 2))
                .overlay(Text(initial).font(.display(28, weight: .semibold)).foregroundStyle(Theme.cream))
                .shadow(color: Theme.rose.opacity(0.25), radius: 16, y: 8)
                .padding(.bottom, 14)

            Text(name).font(.display(22, weight: .semibold)).foregroundStyle(Theme.cream).padding(.bottom, 4)
            Text(email ?? "alex@email.com").font(.amoreBody(13)).foregroundStyle(Theme.muted)

            HStack(spacing: 0) {
                stat("12", "Surprises")
                divider
                stat("3", "Polls sent")
                divider
                stat("∞", "Love given")
            }
            .background(RoundedRectangle(cornerRadius: 18, style: .continuous).fill(Theme.surface))
            .overlay(RoundedRectangle(cornerRadius: 18, style: .continuous).strokeBorder(Theme.border, lineWidth: 1))
            .padding(.top, 24)
        }
        .padding(.horizontal, 24).padding(.bottom, 32)
    }

    private func stat(_ value: String, _ label: String) -> some View {
        VStack(spacing: 3) {
            Text(value).font(.display(20, weight: .bold)).foregroundStyle(Theme.rose)
            Text(label.uppercased()).font(.amoreBody(10)).tracking(0.8).foregroundStyle(Theme.muted)
        }
        .frame(maxWidth: .infinity).padding(.vertical, 14)
    }

    private var divider: some View { Rectangle().fill(Theme.border).frame(width: 1, height: 44) }

    private var menu: some View {
        VStack(spacing: 10) {
            menuRow(icon: "list.bullet.rectangle", tintBg: Theme.rose.opacity(0.15), iconColor: Theme.rose,
                    label: "Generate Poll", sub: "Share with your partner to learn her wishes",
                    labelColor: Theme.cream, action: onGeneratePoll)
            menuRow(icon: "rectangle.portrait.and.arrow.right", tintBg: Color.white.opacity(0.05), iconColor: Theme.muted,
                    label: "Sign Out", sub: nil, labelColor: Theme.muted, action: onSignOut)
        }
        .padding(.horizontal, 24)
    }

    private func menuRow(icon: String, tintBg: Color, iconColor: Color, label: String,
                         sub: String?, labelColor: Color, action: @escaping () -> Void) -> some View {
        Button(action: action) {
            HStack(spacing: 14) {
                RoundedRectangle(cornerRadius: 14, style: .continuous).fill(tintBg)
                    .frame(width: 44, height: 44)
                    .overlay(Image(systemName: icon).font(.system(size: 18)).foregroundStyle(iconColor))
                VStack(alignment: .leading, spacing: 2) {
                    Text(label).font(.amoreBody(14, weight: .medium)).foregroundStyle(labelColor)
                    if let sub { Text(sub).font(.amoreBody(12)).foregroundStyle(Theme.muted).multilineTextAlignment(.leading) }
                }
                Spacer()
                Image(systemName: "chevron.right").font(.system(size: 13, weight: .semibold)).foregroundStyle(Theme.border)
            }
            .padding(16)
            .background(RoundedRectangle(cornerRadius: 18, style: .continuous).fill(Theme.surface))
            .overlay(RoundedRectangle(cornerRadius: 18, style: .continuous).strokeBorder(Theme.border, lineWidth: 1))
        }
        .buttonStyle(PressableStyle())
    }
}
