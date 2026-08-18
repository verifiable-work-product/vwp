import AppKit
import Foundation
import QuickLookUI
import SwiftUI
import UniformTypeIdentifiers
import WebKit

indirect enum JSONValue: Decodable {
    case object([String: JSONValue])
    case array([JSONValue])
    case string(String)
    case number(Double)
    case bool(Bool)
    case null

    init(from decoder: Decoder) throws {
        let container = try decoder.singleValueContainer()
        if container.decodeNil() { self = .null }
        else if let value = try? container.decode(Bool.self) { self = .bool(value) }
        else if let value = try? container.decode(Double.self) { self = .number(value) }
        else if let value = try? container.decode(String.self) { self = .string(value) }
        else if let value = try? container.decode([String: JSONValue].self) { self = .object(value) }
        else if let value = try? container.decode([JSONValue].self) { self = .array(value) }
        else { throw DecodingError.dataCorruptedError(in: container, debugDescription: "Unsupported JSON value") }
    }

    var object: [String: JSONValue]? {
        if case .object(let value) = self { return value }
        return nil
    }

    var string: String? {
        if case .string(let value) = self { return value }
        return nil
    }

    var foundationValue: Any {
        switch self {
        case .object(let value): return value.mapValues(\.foundationValue)
        case .array(let value): return value.map(\.foundationValue)
        case .string(let value): return value
        case .number(let value): return value
        case .bool(let value): return value
        case .null: return NSNull()
        }
    }

    var pretty: String {
        guard JSONSerialization.isValidJSONObject(foundationValue),
              let data = try? JSONSerialization.data(withJSONObject: foundationValue, options: [.prettyPrinted, .sortedKeys]),
              let text = String(data: data, encoding: .utf8) else {
            return String(describing: foundationValue)
        }
        return text
    }
}

struct EmbeddedFile: Decodable, Identifiable {
    let path: String
    let displayName: String
    let mediaType: String
    let role: String
    let extractedPath: String?
    let content: String?
    var id: String { path }
}

struct PartPayload: Decodable, Identifiable {
    let path: String
    let mediaType: String
    let data: JSONValue?
    let content: String?
    var id: String { path }
}

struct VerificationResult: Decodable, Identifiable {
    let validatorId: String
    let status: String
    let message: String?
    let refs: [String]?
    var id: String { validatorId }
}

struct VerificationReport: Decodable {
    let packageOrigin: String
    let results: [VerificationResult]
    let valid: Bool
}

struct VWPDocument: Decodable {
    let sourcePath: String
    let title: String
    let manifest: JSONValue
    let workproduct: JSONValue
    let readable: EmbeddedFile?
    let primaryArtifact: EmbeddedFile?
    let artifacts: [EmbeddedFile]
    let renderings: [EmbeddedFile]
    let facts: [JSONValue]
    let claims: [JSONValue]
    let judgments: [JSONValue]
    let calculations: [PartPayload]
    let evidence: [JSONValue]
    let approvals: [JSONValue]
    let attestations: [PartPayload]
    let provenance: JSONValue
    let verification: VerificationReport
}

final class ViewerStore: ObservableObject {
    static let shared = ViewerStore()

    @Published var document: VWPDocument?
    @Published var isLoading = false
    @Published var errorMessage: String?
    private var activeTemporaryDirectory: URL?

    func chooseFile() {
        let panel = NSOpenPanel()
        panel.title = "Open Verifiable Work Product"
        panel.prompt = "Open"
        panel.allowedContentTypes = [UTType(importedAs: "ac.vwp.package")]
        panel.allowsMultipleSelection = false
        panel.canChooseDirectories = false
        if panel.runModal() == .OK, let url = panel.url {
            open(url)
        }
    }

    func open(_ url: URL) {
        guard url.pathExtension.lowercased() == "vwp" else {
            errorMessage = "VWP Viewer opens .vwp packages."
            return
        }
        isLoading = true
        errorMessage = nil
        let extractionURL = FileManager.default.temporaryDirectory
            .appendingPathComponent("vwp-viewer-\(UUID().uuidString)", isDirectory: true)
        DispatchQueue.global(qos: .userInitiated).async {
            do {
                let result = try HelperRunner.inspect(packageURL: url, extractionURL: extractionURL)
                DispatchQueue.main.async {
                    let previousDirectory = self.activeTemporaryDirectory
                    self.activeTemporaryDirectory = extractionURL
                    self.document = result
                    self.isLoading = false
                    NSDocumentController.shared.noteNewRecentDocumentURL(url)
                    if let previousDirectory { try? FileManager.default.removeItem(at: previousDirectory) }
                }
            } catch {
                DispatchQueue.main.async {
                    try? FileManager.default.removeItem(at: extractionURL)
                    self.errorMessage = error.localizedDescription
                    self.isLoading = false
                }
            }
        }
    }

    func cleanup() {
        if let activeTemporaryDirectory { try? FileManager.default.removeItem(at: activeTemporaryDirectory) }
        activeTemporaryDirectory = nil
    }
}

enum HelperRunner {
    static func inspect(packageURL: URL, extractionURL: URL) throws -> VWPDocument {
        guard let executable = Bundle.main.executableURL else {
            throw ViewerError("Cannot locate the app executable.")
        }
        let helper = executable.deletingLastPathComponent().appendingPathComponent("vwp-viewer-helper")
        guard FileManager.default.isExecutableFile(atPath: helper.path) else {
            throw ViewerError("The VWP verifier helper is missing from this app bundle.")
        }

        let process = Process()
        process.executableURL = helper
        process.arguments = ["inspect", packageURL.path, extractionURL.path]
        let output = Pipe()
        let errors = Pipe()
        process.standardOutput = output
        process.standardError = errors
        try process.run()
        let data = output.fileHandleForReading.readDataToEndOfFile()
        let errorData = errors.fileHandleForReading.readDataToEndOfFile()
        process.waitUntilExit()
        guard process.terminationStatus == 0 else {
            let message = String(data: errorData, encoding: .utf8)?.trimmingCharacters(in: .whitespacesAndNewlines)
            throw ViewerError(message?.isEmpty == false ? message! : "The package could not be opened.")
        }
        do {
            return try JSONDecoder().decode(VWPDocument.self, from: data)
        } catch {
            throw ViewerError("The verifier returned an unreadable report: \(error.localizedDescription)")
        }
    }
}

struct ViewerError: LocalizedError {
    let message: String
    init(_ message: String) { self.message = message }
    var errorDescription: String? { message }
}

final class AppDelegate: NSObject, NSApplicationDelegate {
    func applicationDidFinishLaunching(_ notification: Notification) {
        if let path = CommandLine.arguments.dropFirst().first, path.lowercased().hasSuffix(".vwp") {
            ViewerStore.shared.open(URL(fileURLWithPath: path))
        }
    }

    func application(_ sender: NSApplication, openFiles filenames: [String]) {
        if let filename = filenames.first {
            ViewerStore.shared.open(URL(fileURLWithPath: filename))
            sender.reply(toOpenOrPrint: .success)
        } else {
            sender.reply(toOpenOrPrint: .failure)
        }
    }

    func applicationWillTerminate(_ notification: Notification) {
        ViewerStore.shared.cleanup()
    }
}

@main
struct VWPViewerApp: App {
    @NSApplicationDelegateAdaptor(AppDelegate.self) private var appDelegate
    @StateObject private var store = ViewerStore.shared

    var body: some Scene {
        WindowGroup {
            ContentView(store: store)
        }
        .windowStyle(.titleBar)
        .commands {
            CommandGroup(replacing: .newItem) {
                Button("Open…") { store.chooseFile() }
                    .keyboardShortcut("o", modifiers: .command)
            }
        }
    }
}

enum ViewerTab: String, CaseIterable, Identifiable {
    case artifact = "Artifact"
    case facts = "Facts"
    case claims = "Claims"
    case calculations = "Calculations"
    case evidence = "Evidence"
    case review = "Review"
    case checks = "Verification"
    case package = "Package"

    var id: String { rawValue }
    var symbol: String {
        switch self {
        case .artifact: return "paperclip"
        case .facts: return "list.bullet.rectangle"
        case .claims: return "checkmark.seal"
        case .calculations: return "function"
        case .evidence: return "shippingbox"
        case .review: return "person.crop.circle.badge.checkmark"
        case .checks: return "checklist.checked"
        case .package: return "archivebox"
        }
    }
}

struct ContentView: View {
    @ObservedObject var store: ViewerStore
    @State private var selection: ViewerTab?
    @AppStorage("vwp.viewer.inspectorWidth") private var inspectorWidth = 440.0

    private var effectiveInspectorWidth: Double {
        min(max(inspectorWidth, 320), 760)
    }

    var body: some View {
        Group {
            if store.isLoading {
                VStack(spacing: 14) {
                    ProgressView()
                    Text("Opening and verifying package…")
                        .foregroundStyle(.secondary)
                }
            } else if let document = store.document {
                documentView(document)
            } else {
                emptyView
            }
        }
        .frame(minWidth: 1_240, minHeight: 720)
        .navigationTitle(store.document?.title ?? "VWP Viewer")
        .alert("Couldn’t Open VWP", isPresented: Binding(
            get: { store.errorMessage != nil },
            set: { if !$0 { store.errorMessage = nil } }
        )) {
            Button("OK", role: .cancel) { store.errorMessage = nil }
        } message: {
            Text(store.errorMessage ?? "Unknown error")
        }
        .onChange(of: store.document?.sourcePath) { _ in selection = nil }
    }

    private var emptyView: some View {
        VStack(spacing: 18) {
            Image(systemName: "doc.badge.magnifyingglass")
                .font(.system(size: 62, weight: .thin))
                .foregroundStyle(.secondary)
            Text("Open a Verifiable Work Product")
                .font(.title2.weight(.semibold))
            Text("Double-click a .vwp file, or choose one here.")
                .foregroundStyle(.secondary)
            Button("Open .vwp…") { store.chooseFile() }
                .buttonStyle(.borderedProminent)
                .controlSize(.large)
        }
    }

    private func documentView(_ document: VWPDocument) -> some View {
        VStack(spacing: 0) {
            DocumentHeader(document: document)
            Divider()
            HStack(spacing: 0) {
                List(selection: $selection) {
                    Section("Supporting material") {
                        ForEach(ViewerTab.allCases) { tab in
                            Label(tab.rawValue, systemImage: tab.symbol).tag(tab)
                        }
                    }
                }
                .listStyle(.sidebar)
                .frame(width: 190)
                Divider()
                ReadableView(document: document)
                    .frame(minWidth: 480, maxWidth: .infinity, maxHeight: .infinity)
                    .layoutPriority(1)
                InspectorResizeHandle(width: $inspectorWidth)
                inspectorContent(document)
                    .frame(width: CGFloat(effectiveInspectorWidth))
                    .frame(maxHeight: .infinity)
                    .background(Color(nsColor: .windowBackgroundColor))
            }
        }
    }

    @ViewBuilder
    private func inspectorContent(_ document: VWPDocument) -> some View {
        if let selection {
            switch selection {
            case .artifact:
                ArtifactView(document: document)
            case .facts:
                ItemCollectionView(title: "Facts", subtitle: "Evidence-grounded observations", items: document.facts)
            case .claims:
                ItemCollectionView(title: "Claims", subtitle: "Assertions and conclusions supported by the package", items: document.claims)
            case .calculations:
                PartCollectionView(title: "Calculations", parts: document.calculations)
            case .evidence:
                ItemCollectionView(title: "Evidence", subtitle: "Source records and external references", items: document.evidence)
            case .review:
                ReviewView(document: document)
            case .checks:
                VerificationView(report: document.verification)
            case .package:
                PackageView(document: document)
            }
        } else {
            EmptyStateView(
                title: "Supporting material",
                symbol: "sidebar.right",
                message: "Choose facts, evidence, calculations, or another section in the sidebar to inspect it beside the readable memo."
            )
            .padding(24)
        }
    }
}

struct InspectorResizeHandle: View {
    @Binding var width: Double
    @State private var dragStartWidth: Double?

    private let minimumWidth = 320.0
    private let maximumWidth = 760.0
    private let defaultWidth = 440.0

    var body: some View {
        ZStack {
            Color.clear
            Rectangle()
                .fill(Color(nsColor: .separatorColor))
                .frame(width: 1)
        }
        .frame(width: 9)
        .contentShape(Rectangle())
        .gesture(
            DragGesture(minimumDistance: 1, coordinateSpace: .global)
                .onChanged { value in
                    if dragStartWidth == nil {
                        dragStartWidth = min(max(width, minimumWidth), maximumWidth)
                    }
                    let proposed = (dragStartWidth ?? width) - Double(value.translation.width)
                    width = min(max(proposed, minimumWidth), maximumWidth)
                }
                .onEnded { _ in dragStartWidth = nil }
        )
        .onTapGesture(count: 2) { width = defaultWidth }
        .onHover { hovering in
            (hovering ? NSCursor.resizeLeftRight : NSCursor.arrow).set()
        }
        .onAppear { width = min(max(width, minimumWidth), maximumWidth) }
        .accessibilityLabel("Resize supporting material")
        .accessibilityValue("\(Int(width)) points")
        .help("Drag to resize the supporting-material column. Double-click to reset.")
    }
}

struct DocumentHeader: View {
    let document: VWPDocument

    var body: some View {
        HStack(spacing: 16) {
            VStack(alignment: .leading, spacing: 3) {
                Text(document.title).font(.title3.weight(.semibold)).lineLimit(1)
                Text(URL(fileURLWithPath: document.sourcePath).lastPathComponent)
                    .font(.caption.monospaced()).foregroundStyle(.secondary)
            }
            Spacer()
            StatusPill(valid: document.verification.valid, results: document.verification.results)
        }
        .padding(.horizontal, 20)
        .padding(.vertical, 13)
        .background(.bar)
    }
}

struct StatusPill: View {
    let valid: Bool
    let results: [VerificationResult]

    private var reviewCount: Int { results.filter { $0.status == "reviewRequired" }.count }
    private var label: String {
        if !valid { return "Invalid" }
        if reviewCount > 0 { return "Valid · \(reviewCount) review" }
        return "Valid"
    }

    var body: some View {
        Label(label, systemImage: valid ? (reviewCount > 0 ? "exclamationmark.circle.fill" : "checkmark.circle.fill") : "xmark.octagon.fill")
            .font(.caption.weight(.semibold))
            .foregroundStyle(valid ? (reviewCount > 0 ? Color.orange : Color.green) : Color.red)
            .padding(.horizontal, 10).padding(.vertical, 6)
            .background(.quaternary, in: Capsule())
    }
}

struct ReadableView: View {
    let document: VWPDocument

    var body: some View {
        if let artifact = document.readable {
            VStack(spacing: 0) {
                HStack {
                    VStack(alignment: .leading, spacing: 2) {
                        Text(readableLabel(for: artifact))
                            .font(.headline)
                        Text("\(artifact.displayName) · \(artifact.mediaType)")
                            .font(.caption.monospaced()).foregroundStyle(.secondary)
                    }
                    Spacer()
                    if let path = artifact.extractedPath {
                        Button("Open Externally") { NSWorkspace.shared.open(URL(fileURLWithPath: path)) }
                            .help("Opens the embedded file in its default app. Only do this if you trust its active content.")
                    }
                }
                .padding(16)
                Divider()
                ArtifactPreview(artifact: artifact)
            }
        } else {
            EmptyStateView(
                title: "No readable file",
                symbol: "doc.questionmark",
                message: "This package has no canonical Markdown readable part or legacy rendering. Its structured tabs are still available."
            )
        }
    }

    private func readableLabel(for artifact: EmbeddedFile) -> String {
        if artifact.role == "readable" { return "Canonical Markdown" }
        if artifact.role == "primary-artifact" { return "Embedded work product · v0.1 fallback" }
        return "Human-readable rendering · v0.1 fallback"
    }
}

struct ArtifactView: View {
    let document: VWPDocument

    var body: some View {
        if let artifact = document.primaryArtifact {
            VStack(spacing: 0) {
                HStack {
                    VStack(alignment: .leading, spacing: 2) {
                        Text("Embedded native work product").font(.headline)
                        Text("\(artifact.displayName) · \(artifact.mediaType)")
                            .font(.caption.monospaced()).foregroundStyle(.secondary)
                    }
                    Spacer()
                    if let path = artifact.extractedPath {
                        Button("Open Externally") { NSWorkspace.shared.open(URL(fileURLWithPath: path)) }
                            .help("Opens the embedded file in its default app. Only do this if you trust its active content.")
                    }
                }
                .padding(16)
                Divider()
                ArtifactPreview(artifact: artifact)
            }
        } else {
            EmptyStateView(
                title: "No embedded native artifact",
                symbol: "paperclip",
                message: "The native XLSX, PDF, DOCX, or other source work product is optional. The canonical Markdown readable remains available."
            )
        }
    }
}

struct ArtifactPreview: View {
    let artifact: EmbeddedFile

    var body: some View {
        if artifact.mediaType == "text/markdown", let content = artifact.content {
            MarkdownPreview(content: content)
        } else if (artifact.mediaType == "text/plain" || artifact.mediaType == "text/csv" || artifact.mediaType == "application/json"), let content = artifact.content {
            ScrollView([.vertical, .horizontal]) {
                Text(content).font(.system(.body, design: .monospaced)).textSelection(.enabled)
                    .padding(24).frame(maxWidth: .infinity, alignment: .topLeading)
            }
        } else if artifact.mediaType == "text/html", let path = artifact.extractedPath {
            SafeHTMLPreview(url: URL(fileURLWithPath: path))
        } else if let path = artifact.extractedPath {
            QuickLookPreview(url: URL(fileURLWithPath: path))
        } else {
            EmptyStateView(title: "Preview unavailable", symbol: "eye.slash")
        }
    }
}

struct MarkdownPreview: View {
    let content: String

    var body: some View {
        ScrollView {
            LazyVStack(alignment: .leading, spacing: 0) {
                ForEach(MarkdownParser.parse(content)) { block in
                    MarkdownBlockView(block: block)
                }
            }
            .frame(maxWidth: 860, alignment: .leading)
            .padding(.horizontal, 36)
            .padding(.vertical, 30)
            .frame(maxWidth: .infinity, alignment: .topLeading)
        }
        .background(Color(nsColor: .textBackgroundColor))
    }
}

struct MarkdownBlock: Identifiable {
    enum Kind {
        case heading(level: Int, text: String)
        case paragraph(String)
        case unorderedItem(String)
        case orderedItem(number: String, text: String)
        case quote(String)
        case code(String)
        case rule
    }

    let id: Int
    let kind: Kind
}

enum MarkdownParser {
    static func parse(_ source: String) -> [MarkdownBlock] {
        let normalized = source.replacingOccurrences(of: "\r\n", with: "\n")
        let lines = normalized.components(separatedBy: "\n")
        var blocks: [MarkdownBlock] = []
        var index = 0

        func append(_ kind: MarkdownBlock.Kind) {
            blocks.append(MarkdownBlock(id: blocks.count, kind: kind))
        }

        while index < lines.count {
            let trimmed = lines[index].trimmingCharacters(in: .whitespaces)
            if trimmed.isEmpty {
                index += 1
                continue
            }

            if trimmed.hasPrefix("```") {
                index += 1
                var codeLines: [String] = []
                while index < lines.count && !lines[index].trimmingCharacters(in: .whitespaces).hasPrefix("```") {
                    codeLines.append(lines[index])
                    index += 1
                }
                if index < lines.count { index += 1 }
                append(.code(codeLines.joined(separator: "\n")))
                continue
            }

            if let heading = heading(in: trimmed) {
                append(.heading(level: heading.level, text: heading.text))
                index += 1
                continue
            }

            if isRule(trimmed) {
                append(.rule)
                index += 1
                continue
            }

            if trimmed.hasPrefix(">") {
                var quoteLines: [String] = []
                while index < lines.count {
                    let line = lines[index].trimmingCharacters(in: .whitespaces)
                    guard line.hasPrefix(">") else { break }
                    quoteLines.append(String(line.dropFirst()).trimmingCharacters(in: .whitespaces))
                    index += 1
                }
                append(.quote(quoteLines.joined(separator: " ")))
                continue
            }

            if let item = unorderedItem(in: trimmed) {
                let collected = collectContinuation(lines: lines, index: &index, initial: item)
                append(.unorderedItem(collected))
                continue
            }

            if let item = orderedItem(in: trimmed) {
                let collected = collectContinuation(lines: lines, index: &index, initial: item.text)
                append(.orderedItem(number: item.number, text: collected))
                continue
            }

            var paragraphLines = [trimmed]
            index += 1
            while index < lines.count {
                let next = lines[index].trimmingCharacters(in: .whitespaces)
                if next.isEmpty || startsBlock(next) { break }
                paragraphLines.append(next)
                index += 1
            }
            append(.paragraph(paragraphLines.joined(separator: " ")))
        }

        return blocks
    }

    private static func collectContinuation(lines: [String], index: inout Int, initial: String) -> String {
        var pieces = [initial]
        index += 1
        while index < lines.count {
            let next = lines[index].trimmingCharacters(in: .whitespaces)
            if next.isEmpty || startsBlock(next) { break }
            pieces.append(next)
            index += 1
        }
        return pieces.joined(separator: " ")
    }

    private static func startsBlock(_ line: String) -> Bool {
        line.hasPrefix("```") ||
            line.hasPrefix(">") ||
            heading(in: line) != nil ||
            unorderedItem(in: line) != nil ||
            orderedItem(in: line) != nil ||
            isRule(line)
    }

    private static func heading(in line: String) -> (level: Int, text: String)? {
        let level = line.prefix { $0 == "#" }.count
        guard (1...6).contains(level), line.dropFirst(level).first == " " else { return nil }
        return (level, String(line.dropFirst(level + 1)))
    }

    private static func unorderedItem(in line: String) -> String? {
        for prefix in ["- ", "* ", "+ "] where line.hasPrefix(prefix) {
            return String(line.dropFirst(prefix.count))
        }
        return nil
    }

    private static func orderedItem(in line: String) -> (number: String, text: String)? {
        let digits = line.prefix { $0.isNumber }
        guard !digits.isEmpty else { return nil }
        let suffix = line.dropFirst(digits.count)
        guard suffix.hasPrefix(". ") else { return nil }
        return (String(digits) + ".", String(suffix.dropFirst(2)))
    }

    private static func isRule(_ line: String) -> Bool {
        let compact = line.replacingOccurrences(of: " ", with: "")
        guard compact.count >= 3, let first = compact.first, ["-", "*", "_"].contains(first) else { return false }
        return compact.allSatisfy { $0 == first }
    }
}

struct MarkdownBlockView: View {
    let block: MarkdownBlock

    var body: some View {
        switch block.kind {
        case .heading(let level, let text):
            Text(inline(text))
                .font(headingFont(level))
                .textSelection(.enabled)
                .padding(.top, level == 1 ? 0 : 18)
                .padding(.bottom, level == 1 ? 14 : 8)
        case .paragraph(let text):
            Text(inline(text))
                .font(.body)
                .lineSpacing(4)
                .textSelection(.enabled)
                .padding(.bottom, 13)
        case .unorderedItem(let text):
            HStack(alignment: .firstTextBaseline, spacing: 10) {
                Text("•").frame(width: 16, alignment: .trailing)
                Text(inline(text)).lineSpacing(3).textSelection(.enabled)
            }
            .padding(.bottom, 7)
        case .orderedItem(let number, let text):
            HStack(alignment: .firstTextBaseline, spacing: 10) {
                Text(number).monospacedDigit().frame(width: 28, alignment: .trailing)
                Text(inline(text)).lineSpacing(3).textSelection(.enabled)
            }
            .padding(.bottom, 7)
        case .quote(let text):
            HStack(alignment: .top, spacing: 12) {
                RoundedRectangle(cornerRadius: 1).fill(Color.accentColor.opacity(0.65)).frame(width: 3)
                Text(inline(text)).italic().lineSpacing(3).textSelection(.enabled)
            }
            .padding(.vertical, 9)
            .padding(.horizontal, 12)
            .background(Color.accentColor.opacity(0.08), in: RoundedRectangle(cornerRadius: 6))
            .padding(.bottom, 13)
        case .code(let text):
            ScrollView(.horizontal) {
                Text(text)
                    .font(.system(.callout, design: .monospaced))
                    .textSelection(.enabled)
                    .padding(13)
            }
            .background(Color(nsColor: .controlBackgroundColor), in: RoundedRectangle(cornerRadius: 7))
            .padding(.bottom, 13)
        case .rule:
            Divider().padding(.vertical, 13)
        }
    }

    private func inline(_ source: String) -> AttributedString {
        (try? AttributedString(
            markdown: source,
            options: .init(interpretedSyntax: .inlineOnlyPreservingWhitespace)
        )) ?? AttributedString(source)
    }

    private func headingFont(_ level: Int) -> Font {
        switch level {
        case 1: return .largeTitle.weight(.bold)
        case 2: return .title2.weight(.semibold)
        case 3: return .title3.weight(.semibold)
        default: return .headline
        }
    }
}

struct QuickLookPreview: NSViewRepresentable {
    let url: URL
    func makeNSView(context: Context) -> QLPreviewView {
        let view = QLPreviewView(frame: .zero, style: .normal)!
        view.autostarts = true
        view.previewItem = url as NSURL
        return view
    }
    func updateNSView(_ view: QLPreviewView, context: Context) {
        view.previewItem = url as NSURL
    }
}

struct SafeHTMLPreview: NSViewRepresentable {
    let url: URL
    func makeNSView(context: Context) -> WKWebView {
        let configuration = WKWebViewConfiguration()
        configuration.defaultWebpagePreferences.allowsContentJavaScript = false
        let view = WKWebView(frame: .zero, configuration: configuration)
        view.loadFileURL(url, allowingReadAccessTo: url.deletingLastPathComponent())
        return view
    }
    func updateNSView(_ view: WKWebView, context: Context) {}
}

struct ItemCollectionView: View {
    let title: String
    let subtitle: String
    let items: [JSONValue]

    var body: some View {
        VStack(alignment: .leading, spacing: 0) {
            SectionHeading(title: title, subtitle: subtitle, count: items.count)
            Divider()
            if items.isEmpty {
                EmptyStateView(title: "No \(title.lowercased())", symbol: "tray")
            } else {
                ScrollView {
                    LazyVStack(spacing: 10) {
                        ForEach(Array(items.enumerated()), id: \.offset) { _, item in
                            JSONCard(value: item)
                        }
                    }.padding(18)
                }
            }
        }
    }
}

struct JSONCard: View {
    let value: JSONValue
    @State private var expanded = false

    private var object: [String: JSONValue] { value.object ?? [:] }
    private var identifier: String { object["id"]?.string ?? "Record" }
    private var summary: String {
        for key in ["statement", "title", "question", "conclusion", "topic", "type"] {
            if let text = object[key]?.string, !text.isEmpty { return text }
        }
        return identifier
    }

    var body: some View {
        DisclosureGroup(isExpanded: $expanded) {
            Text(value.pretty).font(.system(.caption, design: .monospaced))
                .textSelection(.enabled).frame(maxWidth: .infinity, alignment: .leading).padding(.top, 10)
        } label: {
            VStack(alignment: .leading, spacing: 5) {
                Text(identifier).font(.caption.monospaced()).foregroundStyle(.secondary)
                Text(summary).lineLimit(expanded ? nil : 3).frame(maxWidth: .infinity, alignment: .leading)
            }.padding(.vertical, 5)
        }
        .padding(14)
        .background(Color(nsColor: .controlBackgroundColor), in: RoundedRectangle(cornerRadius: 10))
        .overlay(RoundedRectangle(cornerRadius: 10).stroke(.quaternary))
    }
}

struct PartCollectionView: View {
    let title: String
    let parts: [PartPayload]

    var body: some View {
        VStack(alignment: .leading, spacing: 0) {
            SectionHeading(title: title, subtitle: "Declared inputs, outputs, and schedules", count: parts.count)
            Divider()
            if parts.isEmpty {
                EmptyStateView(title: "No calculations", symbol: "function")
            } else {
                ScrollView {
                    LazyVStack(spacing: 10) {
                        ForEach(parts) { part in
                            VStack(alignment: .leading, spacing: 9) {
                                Text(part.path).font(.caption.monospaced()).foregroundStyle(.secondary)
                                Text(part.data?.pretty ?? part.content ?? "No inline preview")
                                    .font(.system(.caption, design: .monospaced)).textSelection(.enabled)
                                    .frame(maxWidth: .infinity, alignment: .leading)
                            }
                            .padding(14)
                            .background(Color(nsColor: .controlBackgroundColor), in: RoundedRectangle(cornerRadius: 10))
                            .overlay(RoundedRectangle(cornerRadius: 10).stroke(.quaternary))
                        }
                    }.padding(18)
                }
            }
        }
    }
}

struct ReviewView: View {
    let document: VWPDocument
    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 24) {
                EmbeddedCollection(title: "Judgments", items: document.judgments)
                EmbeddedCollection(title: "Approvals", items: document.approvals)
                if !document.attestations.isEmpty {
                    Text("Attestations").font(.title2.weight(.semibold))
                    ForEach(document.attestations) { part in
                        Text(part.data?.pretty ?? part.content ?? part.path)
                            .font(.system(.caption, design: .monospaced)).textSelection(.enabled)
                    }
                }
            }.padding(22)
        }
    }
}

struct EmbeddedCollection: View {
    let title: String
    let items: [JSONValue]
    var body: some View {
        VStack(alignment: .leading, spacing: 10) {
            Text(title).font(.title2.weight(.semibold))
            if items.isEmpty { Text("None").foregroundStyle(.secondary) }
            ForEach(Array(items.enumerated()), id: \.offset) { _, item in JSONCard(value: item) }
        }
    }
}

struct VerificationView: View {
    let report: VerificationReport

    var body: some View {
        VStack(alignment: .leading, spacing: 0) {
            SectionHeading(title: "Verification", subtitle: "Deterministic checks over structure, hashes, references, and tie-outs", count: report.results.count)
            Divider()
            ScrollView {
                LazyVStack(spacing: 1) {
                    ForEach(report.results) { result in
                        HStack(alignment: .firstTextBaseline, spacing: 12) {
                            Text(statusLabel(result.status))
                                .font(.caption.monospaced().weight(.bold))
                                .foregroundStyle(statusColor(result.status)).frame(width: 62, alignment: .leading)
                            VStack(alignment: .leading, spacing: 3) {
                                Text(result.validatorId).font(.callout.monospaced())
                                if let message = result.message { Text(message).font(.caption).foregroundStyle(.secondary) }
                            }
                            Spacer()
                        }.padding(.horizontal, 18).padding(.vertical, 9)
                        Divider().padding(.leading, 92)
                    }
                }.padding(.vertical, 8)
            }
        }
    }

    private func statusLabel(_ status: String) -> String {
        ["pass": "PASS", "fail": "FAIL", "warning": "WARN", "reviewRequired": "REVIEW", "notRun": "SKIP"][status] ?? status.uppercased()
    }
    private func statusColor(_ status: String) -> Color {
        if status == "pass" { return .green }
        if status == "fail" { return .red }
        if status == "warning" || status == "reviewRequired" { return .orange }
        return .secondary
    }
}

struct PackageView: View {
    let document: VWPDocument
    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 24) {
                JSONBlock(title: "Work product", value: document.workproduct)
                JSONBlock(title: "Manifest", value: document.manifest)
                JSONBlock(title: "Provenance", value: document.provenance)
                if !document.artifacts.isEmpty {
                    VStack(alignment: .leading, spacing: 8) {
                        Text("Embedded artifacts").font(.title2.weight(.semibold))
                        ForEach(document.artifacts) { artifact in
                            Text("\(artifact.path) · \(artifact.mediaType)").font(.caption.monospaced())
                        }
                    }
                }
            }.padding(22)
        }
    }
}

struct JSONBlock: View {
    let title: String
    let value: JSONValue
    var body: some View {
        VStack(alignment: .leading, spacing: 10) {
            Text(title).font(.title2.weight(.semibold))
            Text(value.pretty).font(.system(.caption, design: .monospaced)).textSelection(.enabled)
                .padding(14).frame(maxWidth: .infinity, alignment: .leading)
                .background(Color(nsColor: .controlBackgroundColor), in: RoundedRectangle(cornerRadius: 10))
        }
    }
}

struct SectionHeading: View {
    let title: String
    let subtitle: String
    let count: Int
    var body: some View {
        HStack(alignment: .firstTextBaseline) {
            VStack(alignment: .leading, spacing: 3) {
                Text(title).font(.title2.weight(.semibold))
                Text(subtitle).font(.caption).foregroundStyle(.secondary)
            }
            Spacer()
            Text("\(count)").font(.caption.monospaced()).foregroundStyle(.secondary)
        }.padding(18)
    }
}

struct EmptyStateView: View {
    let title: String
    let symbol: String
    var message: String? = nil

    var body: some View {
        VStack(spacing: 12) {
            Image(systemName: symbol).font(.system(size: 38, weight: .thin)).foregroundStyle(.secondary)
            Text(title).font(.headline)
            if let message { Text(message).foregroundStyle(.secondary).multilineTextAlignment(.center).frame(maxWidth: 460) }
        }.frame(maxWidth: .infinity, maxHeight: .infinity)
    }
}
