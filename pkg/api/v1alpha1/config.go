package v1alpha1

import (
	"context"
	"encoding/base64"
	"fmt"
	"github.com/mitchellh/mapstructure"
	"golang.org/x/sync/errgroup"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"time"
)

type ParsedConfig map[string]string

func (cfg *ParsedConfig) Unmarshal(output any) error {
	c := &mapstructure.DecoderConfig{
		Metadata:         nil,
		Result:           output,
		WeaklyTypedInput: true,
		DecodeHook: mapstructure.ComposeDecodeHookFunc(
			mapstructure.StringToTimeDurationHookFunc(),
			mapstructure.StringToTimeHookFunc(time.RFC3339),
			mapstructure.StringToSliceHookFunc(","),
		),
	}
	decoder, err := mapstructure.NewDecoder(c)
	if err != nil {
		return err
	}

	return decoder.Decode(cfg)
}

func (in *DataConnector) ParseConfig(ctx context.Context, rdr client.Reader) (ParsedConfig, error) {
	cfg := make(ParsedConfig)
	cfg["_fqn"] = fmt.Sprintf("%s.%s", in.GetName(), in.GetNamespace())

	g, ctx := errgroup.WithContext(ctx)

	for _, i := range in.Spec.Config {
		if i.Name == "" {
			continue
		}
		if i.Value != "" {
			cfg[i.Name] = i.Value
			continue
		}
		if i.SecretKeyRef == nil {
			continue
		}
		g.Go(func() error {
			secret := &corev1.Secret{}
			err := rdr.Get(context.TODO(), client.ObjectKey{
				Namespace: in.GetNamespace(),
				Name:      i.SecretKeyRef.Name,
			}, secret)
			if err != nil {
				return fmt.Errorf("failed to get secret %s: %w", i.SecretKeyRef.Name, err)
			}

			val, ok := secret.Data[i.SecretKeyRef.Key]
			if !ok {
				return fmt.Errorf("secret %s does not have key %s", i.SecretKeyRef.Name, i.SecretKeyRef.Key)
			}
			cfg[i.Name] = base64.StdEncoding.EncodeToString(val)
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}
	return cfg, nil
}
