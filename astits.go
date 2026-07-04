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
	MpegTsPacketSize = ts.MpegTsPacketSize
	M2TsPacketSize   = ts.M2TsPacketSize

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
	PESData                    = pes.PESData
	PESHeader                  = pes.PESHeader
	PESOptionalHeader          = pes.PESOptionalHeader
	PESOptionalHeaderExtension = pes.PESOptionalHeaderExtension
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
	EITData                = psi.EITData
	EITDataEvent           = psi.EITDataEvent
	NITData                = psi.NITData
	NITDataTransportStream = psi.NITDataTransportStream
	PATData                = psi.PATData
	PATProgram             = psi.PATProgram
	PMTData                = psi.PMTData
	PMTElementaryStream    = psi.PMTElementaryStream
	PSIData                = psi.PSIData
	PSISection             = psi.PSISection
	PSISectionHeader       = psi.PSISectionHeader
	PSISectionSyntax       = psi.PSISectionSyntax
	PSISectionSyntaxData   = psi.PSISectionSyntaxData
	PSISectionSyntaxHeader = psi.PSISectionSyntaxHeader
	PSITableID             = psi.PSITableID
	SDTData                = psi.SDTData
	SDTDataService         = psi.SDTDataService
	StreamType             = psi.StreamType
	TOTData                = psi.TOTData
)

const (
	PSITableIDBAT                        = psi.PSITableIDBAT
	PSITableIDDIT                        = psi.PSITableIDDIT
	PSITableIDEITEnd                     = psi.PSITableIDEITEnd
	PSITableIDEITStart                   = psi.PSITableIDEITStart
	PSITableIDNITVariant1                = psi.PSITableIDNITVariant1
	PSITableIDNITVariant2                = psi.PSITableIDNITVariant2
	PSITableIDNull                       = psi.PSITableIDNull
	PSITableIDPAT                        = psi.PSITableIDPAT
	PSITableIDPMT                        = psi.PSITableIDPMT
	PSITableIDRST                        = psi.PSITableIDRST
	PSITableIDSDTVariant1                = psi.PSITableIDSDTVariant1
	PSITableIDSDTVariant2                = psi.PSITableIDSDTVariant2
	PSITableIDSIT                        = psi.PSITableIDSIT
	PSITableIDST                         = psi.PSITableIDST
	PSITableIDTDT                        = psi.PSITableIDTDT
	PSITableIDTOT                        = psi.PSITableIDTOT
	PSITableTypeBAT                      = psi.PSITableTypeBAT
	PSITableTypeDIT                      = psi.PSITableTypeDIT
	PSITableTypeEIT                      = psi.PSITableTypeEIT
	PSITableTypeNIT                      = psi.PSITableTypeNIT
	PSITableTypeNull                     = psi.PSITableTypeNull
	PSITableTypePAT                      = psi.PSITableTypePAT
	PSITableTypePMT                      = psi.PSITableTypePMT
	PSITableTypeRST                      = psi.PSITableTypeRST
	PSITableTypeSDT                      = psi.PSITableTypeSDT
	PSITableTypeSIT                      = psi.PSITableTypeSIT
	PSITableTypeST                       = psi.PSITableTypeST
	PSITableTypeTDT                      = psi.PSITableTypeTDT
	PSITableTypeTOT                      = psi.PSITableTypeTOT
	PSITableTypeUnknown                  = psi.PSITableTypeUnknown
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
	DescriptorAC3                         = descriptor.DescriptorAC3
	DescriptorAVCVideo                    = descriptor.DescriptorAVCVideo
	DescriptorComponent                   = descriptor.DescriptorComponent
	DescriptorContent                     = descriptor.DescriptorContent
	DescriptorContentItem                 = descriptor.DescriptorContentItem
	DescriptorDataStreamAlignment         = descriptor.DescriptorDataStreamAlignment
	DescriptorEnhancedAC3                 = descriptor.DescriptorEnhancedAC3
	DescriptorExtendedEvent               = descriptor.DescriptorExtendedEvent
	DescriptorExtendedEventItem           = descriptor.DescriptorExtendedEventItem
	DescriptorExtension                   = descriptor.DescriptorExtension
	DescriptorExtensionSupplementaryAudio = descriptor.DescriptorExtensionSupplementaryAudio
	DescriptorHeader                      = descriptor.DescriptorHeader
	DescriptorISO639LanguageAndAudioType  = descriptor.DescriptorISO639LanguageAndAudioType
	DescriptorLocalTimeOffset             = descriptor.DescriptorLocalTimeOffset
	DescriptorLocalTimeOffsetItem         = descriptor.DescriptorLocalTimeOffsetItem
	DescriptorMaximumBitrate              = descriptor.DescriptorMaximumBitrate
	DescriptorNetworkName                 = descriptor.DescriptorNetworkName
	DescriptorParentalRating              = descriptor.DescriptorParentalRating
	DescriptorParentalRatingItem          = descriptor.DescriptorParentalRatingItem
	DescriptorParser                      = descriptor.DescriptorParser
	DescriptorPrivateDataIndicator        = descriptor.DescriptorPrivateDataIndicator
	DescriptorPrivateDataSpecifier        = descriptor.DescriptorPrivateDataSpecifier
	DescriptorRegistration                = descriptor.DescriptorRegistration
	DescriptorService                     = descriptor.DescriptorService
	DescriptorShortEvent                  = descriptor.DescriptorShortEvent
	DescriptorStreamIdentifier            = descriptor.DescriptorStreamIdentifier
	DescriptorSubtitling                  = descriptor.DescriptorSubtitling
	DescriptorSubtitlingItem              = descriptor.DescriptorSubtitlingItem
	DescriptorTag                         = descriptor.DescriptorTag
	DescriptorTeletext                    = descriptor.DescriptorTeletext
	DescriptorTeletextItem                = descriptor.DescriptorTeletextItem
	DescriptorUnknown                     = descriptor.DescriptorUnknown
	DescriptorUserDefined                 = descriptor.DescriptorUserDefined
	DescriptorVBIData                     = descriptor.DescriptorVBIData
	DescriptorVBIDataDescriptor           = descriptor.DescriptorVBIDataDescriptor
	DescriptorVBIDataService              = descriptor.DescriptorVBIDataService
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
	DescriptorTagAC3                                         = descriptor.DescriptorTagAC3
	DescriptorTagAVCVideo                                    = descriptor.DescriptorTagAVCVideo
	DescriptorTagComponent                                   = descriptor.DescriptorTagComponent
	DescriptorTagContent                                     = descriptor.DescriptorTagContent
	DescriptorTagDataStreamAlignment                         = descriptor.DescriptorTagDataStreamAlignment
	DescriptorTagEnhancedAC3                                 = descriptor.DescriptorTagEnhancedAC3
	DescriptorTagExtendedEvent                               = descriptor.DescriptorTagExtendedEvent
	DescriptorTagExtension                                   = descriptor.DescriptorTagExtension
	DescriptorTagExtensionSupplementaryAudio                 = descriptor.DescriptorTagExtensionSupplementaryAudio
	DescriptorTagISO639LanguageAndAudioType                  = descriptor.DescriptorTagISO639LanguageAndAudioType
	DescriptorTagLocalTimeOffset                             = descriptor.DescriptorTagLocalTimeOffset
	DescriptorTagMaximumBitrate                              = descriptor.DescriptorTagMaximumBitrate
	DescriptorTagNetworkName                                 = descriptor.DescriptorTagNetworkName
	DescriptorTagParentalRating                              = descriptor.DescriptorTagParentalRating
	DescriptorTagPrivateDataIndicator                        = descriptor.DescriptorTagPrivateDataIndicator
	DescriptorTagPrivateDataSpecifier                        = descriptor.DescriptorTagPrivateDataSpecifier
	DescriptorTagRegistration                                = descriptor.DescriptorTagRegistration
	DescriptorTagService                                     = descriptor.DescriptorTagService
	DescriptorTagShortEvent                                  = descriptor.DescriptorTagShortEvent
	DescriptorTagStreamIdentifier                            = descriptor.DescriptorTagStreamIdentifier
	DescriptorTagSubtitling                                  = descriptor.DescriptorTagSubtitling
	DescriptorTagTeletext                                    = descriptor.DescriptorTagTeletext
	DescriptorTagVBIData                                     = descriptor.DescriptorTagVBIData
	DescriptorTagVBITeletext                                 = descriptor.DescriptorTagVBITeletext
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
	DemuxerData   = demux.DemuxerData
	PacketsParser = demux.PacketsParser
	Muxer         = mux.Muxer
	MuxerData     = mux.MuxerData
)

var (
	ErrZeroCopyNextData = demux.ErrZeroCopyNextData
	ErrPIDNotFound      = mux.ErrPIDNotFound
	ErrPIDAlreadyExists = mux.ErrPIDAlreadyExists
	ErrPCRPIDInvalid    = mux.ErrPCRPIDInvalid
	ErrCounterOverflow  = mux.ErrCounterOverflow
)

func NewDemuxer(ctx context.Context, r io.Reader, opts ...func(*Demuxer)) *Demuxer {
	return demux.NewDemuxer(ctx, r, opts...)
}

func DemuxerOptPacketSize(packetSize int) func(*Demuxer) {
	return demux.DemuxerOptPacketSize(packetSize)
}

func DemuxerOptPacketsParser(p PacketsParser) func(*Demuxer) {
	return demux.DemuxerOptPacketsParser(p)
}

func DemuxerOptPacketSkipper(s PacketSkipper) func(*Demuxer) {
	return demux.DemuxerOptPacketSkipper(s)
}

func DemuxerOptSkipErrLimit(count int) func(*Demuxer) {
	return demux.DemuxerOptSkipErrLimit(count)
}

func DemuxerOptZeroCopyPackets(batchPackets uint) func(*Demuxer) {
	return demux.DemuxerOptZeroCopyPackets(batchPackets)
}

func NewMuxer(ctx context.Context, w io.Writer, opts ...func(*Muxer)) *Muxer {
	return mux.NewMuxer(ctx, w, opts...)
}

func MuxerOptTablesRetransmitPeriod(newPeriod int) func(*Muxer) {
	return mux.MuxerOptTablesRetransmitPeriod(newPeriod)
}
