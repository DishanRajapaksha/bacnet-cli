package cli

import (
	"fmt"
	"reflect"

	"github.com/DishanRajapaksha/bacnet-cli/internal/bacnetclient"
	"github.com/DishanRajapaksha/industrial-cli-kit/safety"
)

func (a *App) writeV2(args []string) error {
	fs := a.newFlagSet("write")
	common := addCommonFlags(fs)
	targetFlags := addTargetFlags(fs)
	objectText := fs.String("object", "", "BACnet object as TYPE:INSTANCE")
	propertyText := fs.String("property", "present-value", "property name or numeric identifier")
	arrayIndex := fs.Uint("array-index", uint(^uint32(0)), "array index; defaults to BACnet ARRAY_ALL")
	valueText := fs.String("value", "", "value to write")
	valueType := fs.String("type", "", "string, bool, uint, int, float32, or float64")
	priority := fs.Uint("priority", 16, "BACnet command priority, 1 to 16")
	isNull := fs.Bool("null", false, "write BACnet null, commonly used to relinquish a priority")
	dryRun := fs.Bool("dry-run", false, "print the write plan without sending")
	yes := fs.Bool("yes", false, "perform the write")
	if err := fs.Parse(args); err != nil {
		return err
	}
	mode, err := safety.Resolve(*yes, *dryRun)
	if err != nil {
		return fmt.Errorf("%w: %v", bacnetclient.ErrValidation, err)
	}
	target, err := targetFlags.target()
	if err != nil {
		return err
	}
	object, err := bacnetclient.ParseObjectIdentifier(*objectText)
	if err != nil {
		return err
	}
	property, err := bacnetclient.ParsePropertyIdentifier(*propertyText)
	if err != nil {
		return err
	}
	if *priority < 1 || *priority > 16 {
		return &usageError{message: "--priority must be between 1 and 16"}
	}
	value, err := bacnetclient.ParseWriteValue(*valueType, *valueText, *isNull)
	if err != nil {
		return err
	}
	cfg, err := common.loadConfig()
	if err != nil {
		return err
	}
	valueTypeName := "<nil>"
	if value != nil {
		valueTypeName = reflect.TypeOf(value).String()
	}
	plan := bacnetclient.WritePlan{
		DeviceID: target.DeviceID, Address: target.Address, Object: object, Property: property,
		ArrayIndex: uint32(*arrayIndex), Priority: uint8(*priority), Value: value,
		ValueType: valueTypeName, DryRun: mode == safety.DryRun,
	}
	if mode == safety.DryRun {
		return renderWritePlan(a.out, cfg.Output.Format, plan)
	}
	client, _, err := a.open(common)
	if err != nil {
		return err
	}
	defer client.Close()
	if err := client.WriteProperty(bacnetclient.WriteRequest{
		Target: target, Object: object, Property: property,
		ArrayIndex: uint32(*arrayIndex), Priority: uint8(*priority), Value: value,
	}); err != nil {
		return err
	}
	plan.DryRun = false
	return renderWritePlan(a.out, cfg.Output.Format, plan)
}
