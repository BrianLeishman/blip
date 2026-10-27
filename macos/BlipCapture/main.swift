import AppKit
import ScreenCaptureKit
import CoreMedia
import CoreVideo

// stdout is exclusively fixed-size big-endian RGB565 frames; diagnostics go to stderr.
func diagnostic(_ message: String) {
    FileHandle.standardError.write(Data((message + "\n").utf8))
}

final class Capture: NSObject, SCStreamOutput, SCStreamDelegate {
    let width: Int
    let height: Int
    init(width: Int, height: Int) { self.width = width; self.height = height }
    func stream(_ stream: SCStream, didStopWithError error: Error) {
        diagnostic("Capture stopped: \(error.localizedDescription)")
        exit(1)
    }
    func stream(_ stream: SCStream, didOutputSampleBuffer sample: CMSampleBuffer, of type: SCStreamOutputType) {
        guard type == .screen, sample.isValid,
              let attachments = CMSampleBufferGetSampleAttachmentsArray(sample, createIfNecessary: false) as? [[SCStreamFrameInfo: Any]],
              let status = attachments.first?[.status] as? Int,
              status == SCFrameStatus.complete.rawValue,
              let image = sample.imageBuffer,
              CVPixelBufferGetWidth(image) == width, CVPixelBufferGetHeight(image) == height else { return }
        CVPixelBufferLockBaseAddress(image, .readOnly)
        defer { CVPixelBufferUnlockBaseAddress(image, .readOnly) }
        guard let base = CVPixelBufferGetBaseAddress(image) else { return }
        let stride = CVPixelBufferGetBytesPerRow(image)
        var frame = Data(count: width * height * 2)
        frame.withUnsafeMutableBytes { (out: UnsafeMutableRawBufferPointer) in
            for y in 0..<height {
                let row = base.advanced(by: y * stride).assumingMemoryBound(to: UInt8.self)
                for x in 0..<width {
                    let r = UInt16(row[x * 4 + 2] >> 3)
                    let g = UInt16(row[x * 4 + 1] >> 2)
                    let b = UInt16(row[x * 4] >> 3)
                    let pixel = (r << 11) | (g << 5) | b
                    let offset = (y * width + x) * 2
                    out[offset] = UInt8(pixel >> 8)
                    out[offset + 1] = UInt8(pixel & 0xff)
                }
            }
        }
        do { try FileHandle.standardOutput.write(contentsOf: frame) }
        catch { exit(0) } // Companion closed the pipe.
    }
}

@main @MainActor struct BlipCapture {
    // SCStream delegates are weak; keep the stream, delegate and CGColor alive
    // explicitly until this short-lived helper process exits.
    private static var activeCapture: (SCStream, Capture, CGColor)?
    static func main() async {
        // A command-line launch must establish the AppKit/WindowServer connection
        // before ScreenCaptureKit creates its graphics resources.
        let application = NSApplication.shared
        application.setActivationPolicy(.accessory)
        application.finishLaunching()
        guard CGPreflightScreenCaptureAccess() else {
            if CommandLine.arguments.contains("--request-permission") { _ = CGRequestScreenCaptureAccess() }
            diagnostic("Allow BlipCapture screen recording in System Settings > Privacy & Security > Screen & System Audio Recording, then swipe right again.")
            exit(2)
        }
        do {
            let content = try await SCShareableContent.excludingDesktopWindows(true, onScreenWindowsOnly: false)
            let windows = content.windows.filter {
                let app = $0.owningApplication?.bundleIdentifier ?? ""
                let title = ($0.title ?? "").lowercased()
                return app.hasPrefix("org.mozilla.firefox") && (title.contains("picture-in-picture") || title.contains("picture in picture"))
            }.sorted { $0.windowID < $1.windowID }
            if CommandLine.arguments.contains("--list") {
                let list = windows.map { ["id": Int($0.windowID), "width": Int($0.frame.width), "height": Int($0.frame.height)] }
                FileHandle.standardOutput.write(try JSONSerialization.data(withJSONObject: list, options: [.sortedKeys]))
                return
            }
            guard let window = windows.first else {
                diagnostic("Open a Firefox video in Picture-in-Picture, then swipe right again.")
                exit(3)
            }
            diagnostic("Capturing Firefox Picture-in-Picture (\(Int(window.frame.width))x\(Int(window.frame.height))).")
            let width = CommandLine.arguments.contains("--detail") ? 160 : 80
            let height = width * 3 / 4
            let config = SCStreamConfiguration()
            config.width = width
            config.height = height
            config.pixelFormat = kCVPixelFormatType_32BGRA
            config.minimumFrameInterval = CMTime(value: 1, timescale: 12)
            config.queueDepth = 3
            config.showsCursor = false
            config.scalesToFit = true
            config.preservesAspectRatio = true
            // ScreenCaptureKit's default letterbox placement is not centered.
            let scale = min(Double(width) / window.frame.width, Double(height) / window.frame.height)
            let pictureWidth = floor(window.frame.width * scale)
            let pictureHeight = floor(window.frame.height * scale)
            config.destinationRect = CGRect(
                x: floor((Double(width) - pictureWidth) / 2),
                y: floor((Double(height) - pictureHeight) / 2),
                width: pictureWidth, height: pictureHeight)
            let background = CGColor(gray: 0, alpha: 1)
            config.backgroundColor = background
            config.capturesAudio = false
            config.ignoreShadowsSingleWindow = true
            let output = Capture(width: width, height: height)
            let stream = SCStream(filter: SCContentFilter(desktopIndependentWindow: window), configuration: config, delegate: output)
            try stream.addStreamOutput(output, type: .screen, sampleHandlerQueue: DispatchQueue(label: "blip.capture"))
            activeCapture = (stream, output, background)
            try await stream.startCapture()
            while true { try await Task.sleep(nanoseconds: 1_000_000_000) }
        } catch {
            diagnostic("Capture failed: \(error.localizedDescription)")
            exit(1)
        }
    }
}
