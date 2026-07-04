package astits

import (
	"context"
	"io"

	"github.com/k-danil/go-astits/demux"
	"github.com/k-danil/go-astits/descriptor"
	"github.com/k-danil/go-astits/mux"
	"github.com/k-danil/go-astits/pes"
	"github.com/k-danil/go-astits/psi"
	"github.com/k-danil/go-astits/ts"
)

type (
	Packet                         = ts.Packet
	PacketHeader                   = ts.PacketHeader
	PacketAdaptationField          = ts.PacketAdaptationField
	PacketAdaptationExtensionField = ts.PacketAdaptationExtensionField
	ClockReference                 = ts.ClockReference
	PacketList                     = ts.PacketList
	PacketSkipper                  = ts.PacketSkipper
)

const (
	MpegTsPacketSize = ts.PacketSize
	M2TsPacketSize   = ts.M2TSPacketSize

	ScramblingControlNotScrambled         = ts.ScramblingControlNotScrambled
	ScramblingControlReservedForFutureUse = ts.ScramblingControlReservedForFutureUse
	ScramblingControlScrambledWithEvenKey = ts.ScramblingControlScrambledWithEvenKey
	ScramblingControlScrambledWithOddKey  = ts.ScramblingControlScrambledWithOddKey

	PIDPAT  = ts.PIDPAT
	PIDCAT  = ts.PIDCAT
	PIDTSDT = ts.PIDTSDT
	PIDNull = ts.PIDNull
)

var (
	ErrNoMorePackets                = ts.ErrNoMorePackets
	ErrPacketMustStartWithASyncByte = ts.ErrPacketMustStartWithASyncByte
	ErrShortPacket                  = ts.ErrShortPacket
	EmptySkipper                    = ts.EmptySkipper
)

func NewPacket() *Packet { return ts.NewPacket() }

func NewPacketList() *PacketList { return ts.NewPacketList() }

type (
	DSMTrickMode               = pes.DSMTrickMode
	PESData                    = pes.Data
	PESHeader                  = pes.Header
	PESOptionalHeader          = pes.OptionalHeader
	PESOptionalHeaderExtension = pes.OptionalHeaderExtension
)

const (
	PSTDBufferScale1024Bytes    = pes.PSTDBufferScale1024Bytes
	PSTDBufferScale128Bytes     = pes.PSTDBufferScale128Bytes
	PTSDTSIndicatorBothPresent  = pes.PTSDTSIndicatorBothPresent
	PTSDTSIndicatorIsForbidden  = pes.PTSDTSIndicatorIsForbidden
	PTSDTSIndicatorNoPTSOrDTS   = pes.PTSDTSIndicatorNoPTSOrDTS
	PTSDTSIndicatorOnlyPTS      = pes.PTSDTSIndicatorOnlyPTS
	StreamIDPaddingStream       = pes.StreamIDPaddingStream
	StreamIDPrivateStream1      = pes.StreamIDPrivateStream1
	StreamIDPrivateStream2      = pes.StreamIDPrivateStream2
	TrickModeControlFastForward = pes.TrickModeControlFastForward
	TrickModeControlFastReverse = pes.TrickModeControlFastReverse
	TrickModeControlFreezeFrame = pes.TrickModeControlFreezeFrame
	TrickModeControlSlowMotion  = pes.TrickModeControlSlowMotion
	TrickModeControlSlowReverse = pes.TrickModeControlSlowReverse
)

type (
	EITData                = psi.EIT
	EITDataEvent           = psi.EITEvent
	NITData                = psi.NIT
	NITDataTransportStream = psi.NITTransportStream
	PATData                = psi.PAT
	PATProgram             = psi.PATProgram
	PMTData                = psi.PMT
	PMTElementaryStream    = psi.ElementaryStream
	PSIData                = psi.Data
	PSISection             = psi.Section
	PSISectionHeader       = psi.SectionHeader
	PSISectionSyntax       = psi.SectionSyntax
	PSISectionSyntaxData   = psi.SectionSyntaxData
	PSISectionSyntaxHeader = psi.SectionSyntaxHeader
	PSITableID             = psi.TableID
	SDTData                = psi.SDT
	SDTDataService         = psi.SDTService
	StreamType             = psi.StreamType
	TOTData                = psi.TOT
)

const (
	PSITableIDBAT                        = psi.TableIDBAT
	PSITableIDDIT                        = psi.TableIDDIT
	PSITableIDEITEnd                     = psi.TableIDEITEnd
	PSITableIDEITStart                   = psi.TableIDEITStart
	PSITableIDNITVariant1                = psi.TableIDNITVariant1
	PSITableIDNITVariant2                = psi.TableIDNITVariant2
	PSITableIDNull                       = psi.TableIDNull
	PSITableIDPAT                        = psi.TableIDPAT
	PSITableIDPMT                        = psi.TableIDPMT
	PSITableIDRST                        = psi.TableIDRST
	PSITableIDSDTVariant1                = psi.TableIDSDTVariant1
	PSITableIDSDTVariant2                = psi.TableIDSDTVariant2
	PSITableIDSIT                        = psi.TableIDSIT
	PSITableIDST                         = psi.TableIDST
	PSITableIDTDT                        = psi.TableIDTDT
	PSITableIDTOT                        = psi.TableIDTOT
	PSITableTypeBAT                      = psi.TableTypeBAT
	PSITableTypeDIT                      = psi.TableTypeDIT
	PSITableTypeEIT                      = psi.TableTypeEIT
	PSITableTypeNIT                      = psi.TableTypeNIT
	PSITableTypeNull                     = psi.TableTypeNull
	PSITableTypePAT                      = psi.TableTypePAT
	PSITableTypePMT                      = psi.TableTypePMT
	PSITableTypeRST                      = psi.TableTypeRST
	PSITableTypeSDT                      = psi.TableTypeSDT
	PSITableTypeSIT                      = psi.TableTypeSIT
	PSITableTypeST                       = psi.TableTypeST
	PSITableTypeTDT                      = psi.TableTypeTDT
	PSITableTypeTOT                      = psi.TableTypeTOT
	PSITableTypeUnknown                  = psi.TableTypeUnknown
	RunningStatusNotRunning              = psi.RunningStatusNotRunning
	RunningStatusPausing                 = psi.RunningStatusPausing
	RunningStatusRunning                 = psi.RunningStatusRunning
	RunningStatusServiceOffAir           = psi.RunningStatusServiceOffAir
	RunningStatusStartsInAFewSeconds     = psi.RunningStatusStartsInAFewSeconds
	RunningStatusUndefined               = psi.RunningStatusUndefined
	StreamTypeAACAudio                   = psi.StreamTypeAACAudio
	StreamTypeAACLATMAudio               = psi.StreamTypeAACLATMAudio
	StreamTypeAC3Audio                   = psi.StreamTypeAC3Audio
	StreamTypeADTS                       = psi.StreamTypeADTS
	StreamTypeCAVSVideo                  = psi.StreamTypeCAVSVideo
	StreamTypeDIRACVideo                 = psi.StreamTypeDIRACVideo
	StreamTypeDTSAudio                   = psi.StreamTypeDTSAudio
	StreamTypeEAC3Audio                  = psi.StreamTypeEAC3Audio
	StreamTypeH264Video                  = psi.StreamTypeH264Video
	StreamTypeH265Video                  = psi.StreamTypeH265Video
	StreamTypeHEVCVideo                  = psi.StreamTypeHEVCVideo
	StreamTypeMPEG1Audio                 = psi.StreamTypeMPEG1Audio
	StreamTypeMPEG1Video                 = psi.StreamTypeMPEG1Video
	StreamTypeMPEG2Audio                 = psi.StreamTypeMPEG2Audio
	StreamTypeMPEG2HalvedSampleRateAudio = psi.StreamTypeMPEG2HalvedSampleRateAudio
	StreamTypeMPEG2PacketizedData        = psi.StreamTypeMPEG2PacketizedData
	StreamTypeMPEG2Video                 = psi.StreamTypeMPEG2Video
	StreamTypeMPEG4Video                 = psi.StreamTypeMPEG4Video
	StreamTypeMetadata                   = psi.StreamTypeMetadata
	StreamTypePrivateData                = psi.StreamTypePrivateData
	StreamTypePrivateSection             = psi.StreamTypePrivateSection
	StreamTypeSCTE35                     = psi.StreamTypeSCTE35
	StreamTypeTRUEHDAudio                = psi.StreamTypeTRUEHDAudio
	StreamTypeVC1Video                   = psi.StreamTypeVC1Video
)

type (
	Descriptor                            = descriptor.Descriptor
	DescriptorAC3                         = descriptor.AC3
	DescriptorAVCVideo                    = descriptor.AVCVideo
	DescriptorComponent                   = descriptor.Component
	DescriptorContent                     = descriptor.Content
	DescriptorContentItem                 = descriptor.ContentItem
	DescriptorDataStreamAlignment         = descriptor.DataStreamAlignment
	DescriptorEnhancedAC3                 = descriptor.EnhancedAC3
	DescriptorExtendedEvent               = descriptor.ExtendedEvent
	DescriptorExtendedEventItem           = descriptor.ExtendedEventItem
	DescriptorExtension                   = descriptor.Extension
	DescriptorExtensionSupplementaryAudio = descriptor.ExtensionSupplementaryAudio
	DescriptorHeader                      = descriptor.Header
	DescriptorISO639LanguageAndAudioType  = descriptor.ISO639LanguageAndAudioType
	DescriptorLocalTimeOffset             = descriptor.LocalTimeOffset
	DescriptorLocalTimeOffsetItem         = descriptor.LocalTimeOffsetItem
	DescriptorMaximumBitrate              = descriptor.MaximumBitrate
	DescriptorNetworkName                 = descriptor.NetworkName
	DescriptorParentalRating              = descriptor.ParentalRating
	DescriptorParentalRatingItem          = descriptor.ParentalRatingItem
	DescriptorPrivateDataIndicator        = descriptor.PrivateDataIndicator
	DescriptorPrivateDataSpecifier        = descriptor.PrivateDataSpecifier
	DescriptorRegistration                = descriptor.Registration
	DescriptorService                     = descriptor.Service
	DescriptorShortEvent                  = descriptor.ShortEvent
	DescriptorStreamIdentifier            = descriptor.StreamIdentifier
	DescriptorSubtitling                  = descriptor.Subtitling
	DescriptorSubtitlingItem              = descriptor.SubtitlingItem
	DescriptorTag                         = descriptor.Tag
	DescriptorTeletext                    = descriptor.Teletext
	DescriptorTeletextItem                = descriptor.TeletextItem
	DescriptorUnknown                     = descriptor.Unknown
	DescriptorUserDefined                 = descriptor.UserDefined
	DescriptorVBIData                     = descriptor.VBIData
	DescriptorVBIDataDescriptor           = descriptor.VBIDataDescriptor
	DescriptorVBIDataService              = descriptor.VBIDataService
)

const (
	AudioTypeCleanEffects                                    = descriptor.AudioTypeCleanEffects
	AudioTypeHearingImpaired                                 = descriptor.AudioTypeHearingImpaired
	AudioTypeVisualImpairedCommentary                        = descriptor.AudioTypeVisualImpairedCommentary
	DataStreamAligmentAudioSyncWord                          = descriptor.DataStreamAligmentAudioSyncWord
	DataStreamAligmentVideoAccessUnit                        = descriptor.DataStreamAligmentVideoAccessUnit
	DataStreamAligmentVideoGOPOrSEQ                          = descriptor.DataStreamAligmentVideoGOPOrSEQ
	DataStreamAligmentVideoSEQ                               = descriptor.DataStreamAligmentVideoSEQ
	DataStreamAligmentVideoSliceOrAccessUnit                 = descriptor.DataStreamAligmentVideoSliceOrAccessUnit
	DescriptorTagAC3                                         = descriptor.TagAC3
	DescriptorTagAVCVideo                                    = descriptor.TagAVCVideo
	DescriptorTagComponent                                   = descriptor.TagComponent
	DescriptorTagContent                                     = descriptor.TagContent
	DescriptorTagDataStreamAlignment                         = descriptor.TagDataStreamAlignment
	DescriptorTagEnhancedAC3                                 = descriptor.TagEnhancedAC3
	DescriptorTagExtendedEvent                               = descriptor.TagExtendedEvent
	DescriptorTagExtension                                   = descriptor.TagExtension
	DescriptorTagExtensionSupplementaryAudio                 = descriptor.TagExtensionSupplementaryAudio
	DescriptorTagISO639LanguageAndAudioType                  = descriptor.TagISO639LanguageAndAudioType
	DescriptorTagLocalTimeOffset                             = descriptor.TagLocalTimeOffset
	DescriptorTagMaximumBitrate                              = descriptor.TagMaximumBitrate
	DescriptorTagNetworkName                                 = descriptor.TagNetworkName
	DescriptorTagParentalRating                              = descriptor.TagParentalRating
	DescriptorTagPrivateDataIndicator                        = descriptor.TagPrivateDataIndicator
	DescriptorTagPrivateDataSpecifier                        = descriptor.TagPrivateDataSpecifier
	DescriptorTagRegistration                                = descriptor.TagRegistration
	DescriptorTagService                                     = descriptor.TagService
	DescriptorTagShortEvent                                  = descriptor.TagShortEvent
	DescriptorTagStreamIdentifier                            = descriptor.TagStreamIdentifier
	DescriptorTagSubtitling                                  = descriptor.TagSubtitling
	DescriptorTagTeletext                                    = descriptor.TagTeletext
	DescriptorTagVBIData                                     = descriptor.TagVBIData
	DescriptorTagVBITeletext                                 = descriptor.TagVBITeletext
	ServiceTypeDigitalTelevisionService                      = descriptor.ServiceTypeDigitalTelevisionService
	TeletextTypeAdditionalInformationPage                    = descriptor.TeletextTypeAdditionalInformationPage
	TeletextTypeInitialTeletextPage                          = descriptor.TeletextTypeInitialTeletextPage
	TeletextTypeProgramSchedulePage                          = descriptor.TeletextTypeProgramSchedulePage
	TeletextTypeTeletextSubtitlePage                         = descriptor.TeletextTypeTeletextSubtitlePage
	TeletextTypeTeletextSubtitlePageForHearingImpairedPeople = descriptor.TeletextTypeTeletextSubtitlePageForHearingImpairedPeople
	VBIDataServiceIDClosedCaptioning                         = descriptor.VBIDataServiceIDClosedCaptioning
	VBIDataServiceIDEBUTeletext                              = descriptor.VBIDataServiceIDEBUTeletext
	VBIDataServiceIDInvertedTeletext                         = descriptor.VBIDataServiceIDInvertedTeletext
	VBIDataServiceIDMonochrome442Samples                     = descriptor.VBIDataServiceIDMonochrome442Samples
	VBIDataServiceIDVPS                                      = descriptor.VBIDataServiceIDVPS
	VBIDataServiceIDWSS                                      = descriptor.VBIDataServiceIDWSS
)

type (
	Demuxer       = demux.Demuxer
	DemuxerData   = demux.Data
	PacketsParser = demux.PacketsParser
	Muxer         = mux.Muxer
	MuxerData     = mux.Data
)

var (
	ErrZeroCopyNextData = demux.ErrZeroCopyNextData
	ErrPIDNotFound      = mux.ErrPIDNotFound
	ErrPIDAlreadyExists = mux.ErrPIDAlreadyExists
	ErrPCRPIDInvalid    = mux.ErrPCRPIDInvalid
	ErrCounterOverflow  = mux.ErrCounterOverflow
)

func NewDemuxer(ctx context.Context, r io.Reader, opts ...func(*Demuxer)) *Demuxer {
	return demux.New(ctx, r, opts...)
}

func DemuxerOptPacketSize(packetSize int) func(*Demuxer) {
	return demux.WithPacketSize(packetSize)
}

func DemuxerOptPacketsParser(p PacketsParser) func(*Demuxer) {
	return demux.WithPacketsParser(p)
}

func DemuxerOptPacketSkipper(s PacketSkipper) func(*Demuxer) {
	return demux.WithPacketSkipper(s)
}

func DemuxerOptSkipErrLimit(count int) func(*Demuxer) {
	return demux.WithSkipErrLimit(count)
}

func DemuxerOptZeroCopyPackets(batchPackets uint) func(*Demuxer) {
	return demux.WithZeroCopyPackets(batchPackets)
}

func NewMuxer(ctx context.Context, w io.Writer, opts ...func(*Muxer)) *Muxer {
	return mux.New(ctx, w, opts...)
}

func MuxerOptTablesRetransmitPeriod(newPeriod int) func(*Muxer) {
	return mux.WithTablesRetransmitPeriod(newPeriod)
}
