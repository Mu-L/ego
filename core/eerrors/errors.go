package eerrors

import (
	"context"
	"errors"
	"fmt"
	"io"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	"github.com/gotomicro/ego/internal/ecode"
)

// Error 错误接口
type Error interface {
	error
	WithMetadata(map[string]string) Error
	WithMd(map[string]string) Error
	WithMessage(string) Error
	WithMsg(string) Error
	WithErr(err error) Error
}

const (
	// UnknownReason is unknown reason for error info.
	UnknownReason = ""
	// SupportPackageIsVersion1 this constant should not be referenced by any other code.
	SupportPackageIsVersion1 = true
)

var _ Error = &EgoError{}

type errKey string

var errs = map[errKey]*EgoError{}

// Register 注册错误信息
func Register(egoError *EgoError) {
	errs[errKey(egoError.Reason)] = egoError
}

// Error Error信息
func (x *EgoError) Error() string {
	return fmt.Sprintf("error: code = %d reason = %s message = %s metadata = %v", x.Code, x.Reason, x.Message, x.Metadata)
}

// Is 判断是否为根因错误
func (x *EgoError) Is(err error) bool {
	egoErr, flag := err.(*EgoError)
	if !flag {
		return false
	}
	if x == nil {
		return x == egoErr
	}
	if egoErr == nil {
		return x.Reason == ""
	}
	return x.Reason == egoErr.Reason
}

// GRPCStatus returns the Status represented by se.
func (x *EgoError) GRPCStatus() *status.Status {
	s, _ := status.New(codes.Code(x.Code), x.Message).
		WithDetails(&errdetails.ErrorInfo{
			Reason:   x.Reason,
			Metadata: x.Metadata,
		})
	return s
}

// WithMetadata with an MD formed by the mapping of key, value.
// Deprecated: Will be removed in future versions, use WithMd instead.
func (x *EgoError) WithMetadata(md map[string]string) Error {
	err := proto.Clone(x).(*EgoError)
	err.Metadata = md
	return err
}

// WithMd with an MD formed by the mapping of key, value.
func (x *EgoError) WithMd(md map[string]string) Error {
	err := proto.Clone(x).(*EgoError)
	err.Metadata = md
	return err
}

// WithMessage set message to current EgoError
// Deprecated: Will be removed in future versions, use WithMsg instead.
func (x *EgoError) WithMessage(msg string) Error {
	err := proto.Clone(x).(*EgoError)
	err.Message = msg
	return err
}

// WithMsg set message to current EgoError
func (x *EgoError) WithMsg(msg string) Error {
	err := proto.Clone(x).(*EgoError)
	err.Message = msg
	return err
}

func (x *EgoError) WithErr(err error) Error {
	if err == nil {
		return x
	}

	eErr := proto.Clone(x).(*EgoError)
	switch err {
	case io.EOF:
		eErr.Code = int32(codes.Unknown)
		eErr.Reason = io.EOF.Error()
	case context.DeadlineExceeded:
		eErr.Code = int32(codes.DeadlineExceeded)
		eErr.Reason = context.DeadlineExceeded.Error()
	case context.Canceled:
		eErr.Code = int32(codes.Canceled)
		eErr.Reason = context.Canceled.Error()
	case io.ErrUnexpectedEOF:
		eErr.Code = int32(codes.Internal)
		eErr.Reason = io.ErrUnexpectedEOF.Error()
	default:
		return x
	}

	eErr.Message = err.Error()
	return eErr
}

// New returns an error object for the code, message.
func New(code int, reason, message string) *EgoError {
	return &EgoError{
		Code:    int32(code),
		Message: message,
		Reason:  reason,
	}
}

// ToHTTPStatusCode Get equivalent HTTP status code from x.Code
func (x *EgoError) ToHTTPStatusCode() int {
	return ecode.GrpcToHTTPStatusCode(codes.Code(x.Code))
}

// FromError try to convert an error to *Error.
// It supports wrapped errors.
func FromError(err error) *EgoError {
	if err == nil {
		return nil
	}
	if se := new(EgoError); errors.As(err, &se) {
		return se
	}

	gs, ok := status.FromError(err)
	if ok {
		for _, detail := range gs.Details() {
			switch d := detail.(type) {
			case *errdetails.ErrorInfo:
				e, ok := errs[errKey(d.Reason)]
				if ok {
					return e.WithMsg(gs.Message()).WithMetadata(d.Metadata).(*EgoError)
				}
				return New(
					int(gs.Code()),
					d.Reason,
					gs.Message(),
				).WithMd(d.Metadata).(*EgoError)
			}
		}

		return New(int(gs.Code()), gs.Message(), "")
	}
	return New(int(codes.Unknown), UnknownReason, err.Error())
}
