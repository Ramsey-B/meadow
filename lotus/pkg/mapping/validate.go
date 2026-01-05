package mapping

import (
	"github.com/Ramsey-B/lotus/pkg/errors"
	"github.com/Ramsey-B/lotus/pkg/links"
	"github.com/Ramsey-B/lotus/pkg/models"
	"github.com/Ramsey-B/lotus/pkg/steps"
)

func (m *MappingDefinition) GenerateMappingPlan() error {
	m.Steps = make(map[string]*steps.Step)
	for _, link := range m.Links {
		err := m.validateLink(link)
		if err != nil {
			return err
		}
	}

	return nil
}

func (m *MappingDefinition) validateLink(link links.Link) error {
	// Validate link endpoints.
	//
	// Source must be exactly one of: field_id, step_id, constant.
	// Target must be exactly one of: field_id, step_id.
	sourceCount := 0
	if link.Source.FieldID != "" {
		sourceCount++
	}
	if link.Source.StepID != "" {
		sourceCount++
	}
	if link.Source.Constant != nil {
		sourceCount++
	}
	if sourceCount != 1 {
		return errors.NewMappingError("link source must specify exactly one of field_id, step_id, or constant").AddLink(link.GetLinkID())
	}

	targetCount := 0
	if link.Target.FieldID != "" {
		targetCount++
	}
	if link.Target.StepID != "" {
		targetCount++
	}
	if targetCount != 1 {
		return errors.NewMappingError("link target must specify exactly one of field_id or step_id").AddLink(link.GetLinkID())
	}

	fromType := models.ActionValueType{}
	if link.Source.FieldID != "" {
		field, err := m.SourceFields.GetField(link.Source.FieldID)
		if err != nil {
			return errors.WrapMappingError(err).AddLink(link.GetLinkID())
		}

		fromType = field.GetType()
	}

	if link.Source.StepID != "" {
		step, err := m.createStep(link.Source.StepID)
		if err != nil {
			return errors.WrapMappingError(err).AddLink(link.GetLinkID())
		}

		fromType = step.GetOutputType()
	}

	if link.Source.Constant != nil {
		fromType = models.GetActionValueType(link.Source.Constant)
	}

	if link.Target.FieldID != "" {
		field, err := m.TargetFields.GetField(link.Target.FieldID)
		if err != nil {
			return errors.WrapMappingError(err).AddLink(link.GetLinkID())
		}

		err = models.ValidateActionValueType(fromType, field.GetType())
		if err != nil {
			return errors.WrapMappingError(err).AddLink(link.GetLinkID())
		}
	}

	if link.Target.StepID != "" {
		_, err := m.createStep(link.Target.StepID)
		if err != nil {
			return errors.WrapMappingError(err).AddLink(link.GetLinkID())
		}
	}

	return nil
}

func (m *MappingDefinition) createStep(stepID string) (*steps.Step, error) {
	definition, ok := m.StepDefinitions[stepID]
	if !ok {
		return nil, errors.NewMappingError("no definition found for step").AddStep(stepID)
	}

	step, ok := m.Steps[stepID]
	if ok {
		return step, nil
	}

	parentLinks := m.Links.Filter(func(link links.Link) bool {
		return link.Target.StepID == stepID
	})

	if len(parentLinks) == 0 {
		return nil, errors.NewMappingError("step has no parent links").AddStep(stepID)
	}

	inputTypes := make([]models.ActionValueType, 0)
	for _, link := range parentLinks {
		if link.Source.FieldID != "" {
			field, err := m.SourceFields.GetField(link.Source.FieldID)
			if err != nil {
				return nil, errors.WrapMappingError(err).AddStep(stepID)
			}

			inputTypes = append(inputTypes, field.GetType())
		}

		if link.Source.StepID != "" {
			step, err := m.createStep(link.Source.StepID)
			if err != nil {
				return nil, err
			}

			inputTypes = append(inputTypes, step.GetOutputType())
		}

		if link.Source.Constant != nil {
			inputTypes = append(inputTypes, models.GetActionValueType(link.Source.Constant))
		}
	}

	step, err := steps.NewStep(definition, inputTypes...)
	if err != nil {
		return nil, errors.WrapMappingError(err).AddStep(stepID)
	}

	m.Steps[stepID] = step

	return step, nil
}
